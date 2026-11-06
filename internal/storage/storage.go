// Package storage provides persistence backends for omni-cd.
//
// Each consuming package declares the narrow interface it needs (state.Store,
// auth.UserRepository, ...). Store composes them so a single backend — the
// JSON file layout under /data today, SQLite later — can satisfy all of them.
package storage

import (
	"omni-cd/internal/auth"
	"omni-cd/internal/omniinstance"
	"omni-cd/internal/state"
)

// Store is the full set of persistence operations a backend must implement.
type Store interface {
	state.Store
	auth.UserRepository
	auth.OIDCUserRepository
	auth.SessionRepository
	omniinstance.Store

	// Close releases any resources held by the backend.
	Close() error
}
