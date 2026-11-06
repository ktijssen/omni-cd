package auth

import (
	"errors"
	"time"
)

// ErrNotFound is returned by repository lookups when the record does not exist.
var ErrNotFound = errors.New("not found")

// UserRepository persists local user accounts.
type UserRepository interface {
	ListUsers() ([]User, error)
	// GetUser returns ErrNotFound when no user with that username exists.
	GetUser(username string) (User, error)
	// PutUser creates the user or replaces the existing one with the same username.
	PutUser(u User) error
}

// OIDCUser records an OIDC-authenticated user and their assigned role.
type OIDCUser struct {
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"` // admin | viewer | none
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
}

// OIDCUserRepository persists OIDC users and their roles.
type OIDCUserRepository interface {
	ListOIDCUsers() ([]OIDCUser, error)
	// GetOIDCUser returns ErrNotFound when no user with that email exists.
	GetOIDCUser(email string) (OIDCUser, error)
	// PutOIDCUser creates the user or replaces the existing one with the same email.
	PutOIDCUser(u OIDCUser) error
	// DeleteOIDCUser returns ErrNotFound when no user with that email exists.
	DeleteOIDCUser(email string) error
}

// Session holds per-session metadata. The fields have no JSON tags so the
// persisted format stays compatible with existing sessions.json files.
type Session struct {
	LoginTime   time.Time
	Username    string // login identifier: username for local, email for OIDC
	DisplayName string // shown in the sidebar
	AuthMethod  string // "local" or "oidc"
	Role        string // "admin", "viewer", "none" — empty means local-auth default (admin)
}

// SessionRepository persists login sessions, keyed by session token.
type SessionRepository interface {
	LoadSessions() (map[string]Session, error)
	// SaveSessions replaces all stored sessions with the given set.
	SaveSessions(sessions map[string]Session) error
}
