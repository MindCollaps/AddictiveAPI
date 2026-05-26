package ws

import (
	"sync"
	"time"

	"addictiveapi/internal/auth"
)

type Session struct {
	Claims    *auth.Claims
	Token     string
	ExpiresAt time.Time
	done      chan struct{}

	mu sync.RWMutex
}

type Renewal struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewSession(claims *auth.Claims, token string, expiresAt time.Time) *Session {
	return &Session{
		Claims:    claims,
		Token:     token,
		ExpiresAt: expiresAt,
		done:      make(chan struct{}),
	}
}

func (s *Session) UpdateRenewal(token string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Token = token
	s.ExpiresAt = expiresAt
}

func (s *Session) Snapshot() (string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Token, s.ExpiresAt
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}
