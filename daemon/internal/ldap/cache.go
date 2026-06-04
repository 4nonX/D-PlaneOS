package ldap

import (
	"log"
	"sync"
	"time"
)

// DirectoryCache is an in-memory cache of LDAP users and their group memberships,
// populated by a background refresh goroutine. It serves two purposes:
//
//  1. Performance: NSS-style lookups on every file stat() would require a round
//     trip to the directory server. The cache satisfies these from memory.
//  2. Resilience: when the directory server is temporarily unreachable (network
//     glitch, DC rebooting), authentication and role mapping continue to work
//     against the last-known-good snapshot.
//
// Mirrors TrueNAS's DSCacheFill pattern in plugins/directoryservices_/cache.py.
type DirectoryCache struct {
	mu          sync.RWMutex
	users       map[string]*User // key: lowercase username
	groups      map[string][]string // key: lowercase group CN, value: member usernames
	lastRefresh time.Time
	ttl         time.Duration
	stale       bool // true if last refresh failed

	stop chan struct{}
}

// NewDirectoryCache creates a cache with the given TTL. Call Start() to begin
// background refresh.
func NewDirectoryCache(ttl time.Duration) *DirectoryCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &DirectoryCache{
		users:  make(map[string]*User),
		groups: make(map[string][]string),
		ttl:    ttl,
		stop:   make(chan struct{}),
	}
}

// Start launches a background goroutine that refreshes the cache every ttl/2.
// The refresh uses the provided fetch function which should perform the actual
// LDAP query. Call Stop() to shut down the goroutine.
func (c *DirectoryCache) Start(fetch func() ([]*User, error)) {
	go func() {
		// Initial fill on startup
		c.refresh(fetch)

		// Subsequent refreshes at half the TTL so cached data never expires
		// during normal operation (refresh happens before the TTL is reached)
		ticker := time.NewTicker(c.ttl / 2)
		defer ticker.Stop()
		for {
			select {
			case <-c.stop:
				return
			case <-ticker.C:
				c.refresh(fetch)
			}
		}
	}()
}

// Stop shuts down the background refresh goroutine.
func (c *DirectoryCache) Stop() {
	close(c.stop)
}

// refresh calls fetch and atomically replaces the cached data.
func (c *DirectoryCache) refresh(fetch func() ([]*User, error)) {
	users, err := fetch()
	if err != nil {
		c.mu.Lock()
		c.stale = true
		c.mu.Unlock()
		log.Printf("LDAP CACHE: refresh failed (%v); continuing to serve stale data", err)
		return
	}

	newUsers := make(map[string]*User, len(users))
	newGroups := make(map[string][]string)

	for _, u := range users {
		key := lowerASCII(u.Username)
		newUsers[key] = u
		for _, g := range u.Groups {
			gk := lowerASCII(g)
			newGroups[gk] = append(newGroups[gk], u.Username)
		}
	}

	c.mu.Lock()
	c.users = newUsers
	c.groups = newGroups
	c.lastRefresh = time.Now()
	c.stale = false
	c.mu.Unlock()

	log.Printf("LDAP CACHE: refreshed %d users, %d groups", len(newUsers), len(newGroups))
}

// GetUser returns a cached user by username. Returns nil, false if not cached.
func (c *DirectoryCache) GetUser(username string) (*User, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	u, ok := c.users[lowerASCII(username)]
	return u, ok
}

// GetGroupMembers returns the cached member list for a group.
func (c *DirectoryCache) GetGroupMembers(groupCN string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	members, ok := c.groups[lowerASCII(groupCN)]
	return members, ok
}

// AllUsers returns a snapshot of all cached users.
func (c *DirectoryCache) AllUsers() []*User {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*User, 0, len(c.users))
	for _, u := range c.users {
		out = append(out, u)
	}
	return out
}

// IsStale returns true if the last refresh failed (directory unreachable).
// Callers can surface this as a warning without blocking authentication.
func (c *DirectoryCache) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stale
}

// LastRefresh returns the time of the last successful cache fill.
func (c *DirectoryCache) LastRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastRefresh
}

// Expired returns true if the cache has never been filled or the TTL has elapsed
// since the last successful fill. When expired, callers should attempt a live query
// instead of serving cached data.
func (c *DirectoryCache) Expired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastRefresh.IsZero() {
		return true
	}
	return time.Since(c.lastRefresh) > c.ttl
}

// lowerASCII returns a lowercase version of s restricted to ASCII.
// Avoids importing unicode packages for simple case folding on identifiers.
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
