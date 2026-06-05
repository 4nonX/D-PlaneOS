package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"dplaned/internal/middleware"
)

const sseTicketTTL = 30 * time.Second

type sseTicketEntry struct {
	user   *middleware.User
	expiry time.Time
}

var (
	sseTicketMu sync.Mutex
	sseTickets  = map[string]sseTicketEntry{}
)

// MintSSETicket issues a one-time 30-second ticket for an SSE connection.
// The caller must already be authenticated via normal session headers.
// POST /api/sse/ticket
func MintSSETicket(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r)
	if !ok || user == nil {
		respondErrorSimple(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		respondErrorSimple(w, "failed to generate ticket", http.StatusInternalServerError)
		return
	}
	ticket := hex.EncodeToString(b)

	sseTicketMu.Lock()
	// Prune expired entries on each mint to bound memory growth
	now := time.Now()
	for k, v := range sseTickets {
		if now.After(v.expiry) {
			delete(sseTickets, k)
		}
	}
	sseTickets[ticket] = sseTicketEntry{user: user, expiry: now.Add(sseTicketTTL)}
	sseTicketMu.Unlock()

	respondOK(w, map[string]any{"ticket": ticket})
}

// ConsumeSSETicket validates and atomically consumes a one-time SSE ticket.
// Returns the associated user or nil if the ticket is invalid or expired.
func ConsumeSSETicket(ticket string) *middleware.User {
	if ticket == "" {
		return nil
	}
	sseTicketMu.Lock()
	defer sseTicketMu.Unlock()
	entry, ok := sseTickets[ticket]
	if !ok {
		return nil
	}
	delete(sseTickets, ticket) // single-use: consumed on first call
	if time.Now().After(entry.expiry) {
		return nil
	}
	return entry.user
}
