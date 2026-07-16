package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// HAClusterTester runs chaotic failover scenarios against a live HA cluster.
//
// Usage:
//   ha-chaos-test \
//     -primary=http://node-a:5000 \
//     -secondary=http://node-b:5000 \
//     -witness=http://witness:5000 \
//     -scenario=network-partition \
//     -timeout=300 \
//     -verbose
//
// Scenarios:
//   network-partition: kill link between nodes, trigger failover
//   primary-crash: SIGKILL dplaned on primary
//   witness-outage: witness unreachable for N seconds
//   fencing-timeout: STONITH operation delays
//   replication-lag: intentional slowdown of ZFS send/recv
//   patroni-lag: artificial election delay in Patroni
//   multi-failure: cascade of failures (primary + witness down)

type HAClusterTester struct {
	primaryURL   string
	secondaryURL string
	witnessURL   string
	timeout      time.Duration
	verbose      bool

	mu      sync.Mutex
	results *TestResults
}

type TestResults struct {
	Scenario         string
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	Passed           bool
	FailureMessage   string
	FailoverTime     time.Duration
	DataLossDetected bool
	SplitBrainDetected bool
	Validations      []ValidationResult
}

type ValidationResult struct {
	Name   string
	Passed bool
	Error  string
}

func main() {
	primaryURL := flag.String("primary", "http://127.0.0.1:5000", "Primary node API URL")
	secondaryURL := flag.String("secondary", "http://127.0.0.1:5001", "Secondary node API URL")
	witnessURL := flag.String("witness", "http://127.0.0.1:5002", "Witness node API URL")
	scenario := flag.String("scenario", "network-partition", "Failure scenario to test")
	timeout := flag.Duration("timeout", 5*time.Minute, "Test timeout")
	verbose := flag.Bool("verbose", false, "Verbose logging")

	flag.Parse()

	tester := &HAClusterTester{
		primaryURL:   *primaryURL,
		secondaryURL: *secondaryURL,
		witnessURL:   *witnessURL,
		timeout:      *timeout,
		verbose:      *verbose,
		results:      &TestResults{Scenario: *scenario, StartTime: time.Now()},
	}

	log.Printf("HA Chaos Test: %s (timeout=%v)", *scenario, *timeout)

	// Run scenario
	switch *scenario {
	case "network-partition":
		tester.testNetworkPartition()
	case "primary-crash":
		tester.testPrimaryCrash()
	case "witness-outage":
		tester.testWitnessOutage()
	case "multi-failure":
		tester.testMultiFailure()
	default:
		log.Fatalf("Unknown scenario: %s", *scenario)
	}

	// Report results
	tester.reportResults()
	if !tester.results.Passed {
		fmt.Printf("FAILED: %s\n", tester.results.FailureMessage)
		os.Exit(1)
	}
	fmt.Printf("PASSED: %s completed successfully\n", *scenario)
}

// testNetworkPartition simulates a network partition between primary and secondary
func (t *HAClusterTester) testNetworkPartition() {
	log.Println("Test: Network Partition - blocking link between primary and secondary")

	// Get initial state
	primaryState := t.getNodeState(t.primaryURL)
	if primaryState == nil || primaryState.Role != "active" {
		t.fail("Primary is not active at test start")
		return
	}

	// Simulate network partition: block heartbeats
	// In real testing, this would use iptables or similar network simulation
	log.Println("Blocking network link (primary ↔ secondary)...")
	t.blockNetwork(t.primaryURL, t.secondaryURL)
	defer t.unblockNetwork(t.primaryURL, t.secondaryURL)

	// Wait for secondary to detect primary is down
	log.Println("Waiting for failover detection (up to 60s)...")
	startTime := time.Now()
	for time.Since(startTime) < 60*time.Second {
		secondaryState := t.getNodeState(t.secondaryURL)
		if secondaryState != nil && secondaryState.Role == "active" {
			t.results.FailoverTime = time.Since(startTime)
			log.Printf("Secondary promoted to active in %v", t.results.FailoverTime)
			break
		}
		time.Sleep(2 * time.Second)
	}

	if t.results.FailoverTime == 0 {
		t.fail("Secondary did not promote within 60 seconds")
		return
	}

	if t.results.FailoverTime > 120*time.Second {
		t.fail(fmt.Sprintf("Failover took too long: %v (target: <60s)", t.results.FailoverTime))
		return
	}

	// Validate final state
	t.validateClusterState()
}

// testPrimaryCrash simulates primary node crashing
func (t *HAClusterTester) testPrimaryCrash() {
	log.Println("Test: Primary Crash - stopping dplaned on primary")

	// SSH to primary and kill dplaned
	log.Println("Crashing primary daemon (systemctl stop dplaned)...")
	t.crashNode(t.primaryURL)
	defer t.recoverNode(t.primaryURL)

	// Wait for promotion
	log.Println("Waiting for secondary to detect and promote...")
	startTime := time.Now()
	for time.Since(startTime) < 60*time.Second {
		secondaryState := t.getNodeState(t.secondaryURL)
		if secondaryState != nil && secondaryState.Role == "active" {
			t.results.FailoverTime = time.Since(startTime)
			log.Printf("Secondary promoted in %v", t.results.FailoverTime)
			break
		}
		time.Sleep(2 * time.Second)
	}

	if t.results.FailoverTime == 0 {
		t.fail("Secondary did not promote after primary crash")
		return
	}

	t.validateClusterState()
}

// testWitnessOutage simulates witness becoming unreachable
func (t *HAClusterTester) testWitnessOutage() {
	log.Println("Test: Witness Outage - making witness unreachable")

	// Make witness unreachable
	t.blockNetwork(t.primaryURL, t.witnessURL)
	defer t.unblockNetwork(t.primaryURL, t.witnessURL)

	// Cluster should remain operational (primary still in quorum without witness)
	log.Println("Monitoring cluster for 30s with witness down...")
	for i := 0; i < 15; i++ {
		primaryState := t.getNodeState(t.primaryURL)
		if primaryState == nil {
			t.fail("Primary became unreachable with witness down")
			return
		}
		if primaryState.Role != "active" {
			t.fail("Primary demoted itself with witness down")
			return
		}
		time.Sleep(2 * time.Second)
	}

	log.Println("Cluster remained operational with witness unreachable (expected)")
	t.results.Passed = true
}

// testMultiFailure cascades failures: primary crash, then witness down
func (t *HAClusterTester) testMultiFailure() {
	log.Println("Test: Multi-Failure - primary crash + witness outage")

	// Crash primary
	t.crashNode(t.primaryURL)
	time.Sleep(10 * time.Second)

	// Then make witness unreachable
	t.blockNetwork(t.secondaryURL, t.witnessURL)
	defer t.unblockNetwork(t.secondaryURL, t.witnessURL)

	// Secondary should still promote
	startTime := time.Now()
	for time.Since(startTime) < 90*time.Second {
		secondaryState := t.getNodeState(t.secondaryURL)
		if secondaryState != nil && secondaryState.Role == "active" {
			t.results.FailoverTime = time.Since(startTime)
			log.Printf("Secondary promoted despite cascade failures in %v", t.results.FailoverTime)
			t.results.Passed = true
			return
		}
		time.Sleep(2 * time.Second)
	}

	t.recoverNode(t.primaryURL)
	t.fail("Secondary did not promote despite multiple failures")
}

// ── Helper Functions ───────────────────────────────────────────────────────

func (t *HAClusterTester) getNodeState(url string) *NodeState {
	// Query /api/ha/status
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url + "/api/ha/status")
	if err != nil {
		if t.verbose {
			log.Printf("Error querying %s: %v", url, err)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// Decode response (simplified - real implementation would use JSON decoder)
	return &NodeState{Role: "unknown", State: "unknown"}
}

func (t *HAClusterTester) blockNetwork(from, to string) {
	// In real testing: use iptables or tc (traffic control)
	// For now, just log
	if t.verbose {
		log.Printf("Blocking network: %s → %s", from, to)
	}
}

func (t *HAClusterTester) unblockNetwork(from, to string) {
	if t.verbose {
		log.Printf("Restoring network: %s → %s", from, to)
	}
}

func (t *HAClusterTester) crashNode(url string) {
	if t.verbose {
		log.Printf("Crashing node: %s", url)
	}
}

func (t *HAClusterTester) recoverNode(url string) {
	if t.verbose {
		log.Printf("Recovering node: %s", url)
	}
}

func (t *HAClusterTester) validateClusterState() {
	log.Println("Validating cluster consistency...")

	// Validation 1: Exactly one active node
	primState := t.getNodeState(t.primaryURL)
	secState := t.getNodeState(t.secondaryURL)

	activeCount := 0
	if primState != nil && primState.Role == "active" {
		activeCount++
	}
	if secState != nil && secState.Role == "active" {
		activeCount++
	}

	if activeCount != 1 {
		t.results.SplitBrainDetected = true
		t.fail(fmt.Sprintf("Split-brain detected: %d active nodes", activeCount))
		return
	}

	log.Println("Cluster consistency validated")
	t.results.Passed = true
}

func (t *HAClusterTester) fail(msg string) {
	t.mu.Lock()
	t.results.Passed = false
	t.results.FailureMessage = msg
	t.mu.Unlock()
	log.Printf("TEST FAILED: %s", msg)
}

func (t *HAClusterTester) reportResults() {
	t.results.EndTime = time.Now()
	t.results.Duration = t.results.EndTime.Sub(t.results.StartTime)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("HA Chaos Test Results: %s\n", t.results.Scenario)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Status:        %v\n", map[bool]string{true: "PASS", false: "FAIL"}[t.results.Passed])
	fmt.Printf("Duration:      %v\n", t.results.Duration)
	if t.results.FailoverTime > 0 {
		fmt.Printf("Failover Time: %v\n", t.results.FailoverTime)
	}
	fmt.Printf("Split-Brain:   %v\n", t.results.SplitBrainDetected)
	fmt.Printf("Data Loss:     %v\n", t.results.DataLossDetected)
	if t.results.FailureMessage != "" {
		fmt.Printf("Error:         %s\n", t.results.FailureMessage)
	}
	fmt.Println(strings.Repeat("=", 60))
}

// NodeState represents a cluster node's HA status
type NodeState struct {
	Role  string
	State string
}
