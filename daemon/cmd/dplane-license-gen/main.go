package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

func main() {
	customer := flag.String("customer", "", "Customer name")
	audits := flag.String("audits", "unlimited", "Audit limit (number or 'unlimited')")
	expires := flag.String("expires", "never", "Expiration date (YYYY-MM-DD or 'never')")
	ceRepo := flag.String("ce-repo", "", "CE repository URL (e.g., https://github.com/user/repo)")
	ceVersion := flag.String("ce-version", "v6.1.0", "CE version to download")
	ceToken := flag.String("ce-token", "", "GitHub PAT for private repo (optional)")
	privKeyFile := flag.String("private-key", "", "Path to Ed25519 private key (base64-encoded, required)")
	outFile := flag.String("out", "", "Output file (optional, prints to stdout if not set)")

	flag.Parse()

	if *customer == "" {
		log.Fatal("--customer is required")
	}
	if *ceRepo == "" {
		log.Fatal("--ce-repo is required")
	}
	if *privKeyFile == "" {
		log.Fatal("--private-key is required")
	}

	privKeyBytes, err := os.ReadFile(*privKeyFile)
	if err != nil {
		log.Fatalf("Failed to read private key file: %v", err)
	}

	privKeyBase64 := string(privKeyBytes)
	privKeyDecode, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		log.Fatalf("Private key is not valid base64: %v", err)
	}

	if len(privKeyDecode) != ed25519.PrivateKeySize {
		log.Fatalf("Private key size is %d, expected %d", len(privKeyDecode), ed25519.PrivateKeySize)
	}

	privKey := ed25519.PrivateKey(privKeyDecode)

	auditsLimit := -1
	if *audits != "unlimited" {
		parsed, err := strconv.Atoi(*audits)
		if err != nil {
			log.Fatalf("Invalid audits value: %s", *audits)
		}
		auditsLimit = parsed
	}

	expiresAt := "never"
	if *expires != "never" {
		t, err := time.Parse("2006-01-02", *expires)
		if err != nil {
			log.Fatalf("Invalid expiration date format (use YYYY-MM-DD): %s", *expires)
		}
		expiresAt = t.Format(time.RFC3339)
	}

	payload := map[string]interface{}{
		"customer":        *customer,
		"audits_limit":    auditsLimit,
		"expires_at":      expiresAt,
		"ce_repo_url":     *ceRepo,
		"ce_version":      *ceVersion,
		"ce_access_token": *ceToken,
		"issued_at":       time.Now().Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	sig := ed25519.Sign(privKey, payloadBytes)

	licenseKey := base64.StdEncoding.EncodeToString(sig) + "." + base64.StdEncoding.EncodeToString(payloadBytes)

	if *outFile != "" {
		err = os.WriteFile(*outFile, []byte(licenseKey), 0600)
		if err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Printf("License key written to: %s\n", *outFile)
		fmt.Printf("Customer: %s\n", *customer)
		fmt.Printf("Audits: %s\n", *audits)
		fmt.Printf("Expires: %s\n", *expires)
	} else {
		fmt.Println(licenseKey)
	}
}
