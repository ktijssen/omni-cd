package web

import (
	"path/filepath"
	"testing"
	"time"
)

func newOIDCStore(t *testing.T) *oidcUserStore {
	t.Helper()
	return loadOIDCUserStore(filepath.Join(t.TempDir(), "oidc-users.json"))
}

func TestOIDCUserStore_Upsert_NewUser(t *testing.T) {
	s := newOIDCStore(t)
	role := s.upsert("alice@example.com", "Alice", "admin")
	if role != "admin" {
		t.Errorf("upsert should return provided role for new user, got %q", role)
	}
	users := s.list()
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	u := users[0]
	if u.Email != "alice@example.com" {
		t.Errorf("Email: got %q", u.Email)
	}
	if u.Role != "admin" {
		t.Errorf("Role: got %q, want admin", u.Role)
	}
	if u.DisplayName != "Alice" {
		t.Errorf("DisplayName: got %q", u.DisplayName)
	}
	if u.FirstSeen.IsZero() {
		t.Error("FirstSeen should not be zero")
	}
}

func TestOIDCUserStore_Upsert_ExistingUserPreservesRole(t *testing.T) {
	s := newOIDCStore(t)
	s.upsert("alice@example.com", "Alice", "admin")
	// Second call with a different role — stored role must be returned.
	role := s.upsert("alice@example.com", "Alice", "viewer")
	if role != "admin" {
		t.Errorf("upsert of existing user should return stored role 'admin', got %q", role)
	}
	if s.list()[0].Role != "admin" {
		t.Errorf("stored role must remain 'admin'")
	}
}

func TestOIDCUserStore_Upsert_UpdatesLastSeen(t *testing.T) {
	s := newOIDCStore(t)
	s.upsert("alice@example.com", "Alice", "admin")
	firstSeen := s.list()[0].FirstSeen
	lastSeenBefore := s.list()[0].LastSeen

	time.Sleep(2 * time.Millisecond)
	s.upsert("alice@example.com", "Alice", "admin")

	u := s.list()[0]
	if !u.LastSeen.After(lastSeenBefore) {
		t.Error("LastSeen should be updated after second upsert")
	}
	if !u.FirstSeen.Equal(firstSeen) {
		t.Error("FirstSeen should not change on subsequent upsert")
	}
}

func TestOIDCUserStore_SetRole_Found(t *testing.T) {
	s := newOIDCStore(t)
	s.upsert("alice@example.com", "Alice", "viewer")
	ok := s.setRole("alice@example.com", "admin")
	if !ok {
		t.Error("setRole should return true for existing user")
	}
	if s.list()[0].Role != "admin" {
		t.Errorf("role should be updated to 'admin', got %q", s.list()[0].Role)
	}
}

func TestOIDCUserStore_SetRole_NotFound(t *testing.T) {
	s := newOIDCStore(t)
	ok := s.setRole("nobody@example.com", "admin")
	if ok {
		t.Error("setRole should return false for unknown user")
	}
}

func TestOIDCUserStore_Delete(t *testing.T) {
	s := newOIDCStore(t)
	s.upsert("alice@example.com", "Alice", "admin")

	ok := s.delete("alice@example.com")
	if !ok {
		t.Error("delete should return true for existing user")
	}
	if len(s.list()) != 0 {
		t.Error("user should be removed from store")
	}
	// Second delete must return false.
	if s.delete("alice@example.com") {
		t.Error("delete of non-existent user should return false")
	}
}

func TestOIDCUserStore_List_ReturnsCopy(t *testing.T) {
	s := newOIDCStore(t)
	s.upsert("alice@example.com", "Alice", "admin")

	users := s.list()
	users[0].Role = "hacked"

	// Internal store must not be affected.
	if s.list()[0].Role == "hacked" {
		t.Error("list() should return a copy; mutations must not affect the store")
	}
}

func TestOIDCUserStore_SaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oidc-users.json")

	s1 := loadOIDCUserStore(path)
	s1.upsert("alice@example.com", "Alice", "admin")
	s1.upsert("bob@example.com", "Bob", "viewer")

	s2 := loadOIDCUserStore(path)
	users := s2.list()
	if len(users) != 2 {
		t.Fatalf("expected 2 users after reload, got %d", len(users))
	}
	byEmail := make(map[string]oidcUser)
	for _, u := range users {
		byEmail[u.Email] = u
	}
	if byEmail["alice@example.com"].Role != "admin" {
		t.Errorf("alice's role should be admin after reload, got %q", byEmail["alice@example.com"].Role)
	}
	if byEmail["bob@example.com"].Role != "viewer" {
		t.Errorf("bob's role should be viewer after reload, got %q", byEmail["bob@example.com"].Role)
	}
}

func TestOIDCUserStore_IsEmpty(t *testing.T) {
	s := newOIDCStore(t)
	if !s.isEmpty() {
		t.Error("new store should be empty")
	}
	s.upsert("alice@example.com", "Alice", "admin")
	if s.isEmpty() {
		t.Error("store should not be empty after upsert")
	}
}
