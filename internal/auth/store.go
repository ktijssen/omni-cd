package auth

import (
	"errors"
	"log/slog"
	"sync"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// User holds a single account's credentials.
type User struct {
	Username     string `json:"username,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	PasswordHash string `json:"passwordHash"`
}

// UserInfo is a safe, hash-free view of a User for API responses.
type UserInfo struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Store manages local user accounts with bcrypt passwords on top of a
// UserRepository. The mutex serialises read-modify-write updates.
type Store struct {
	mu   sync.Mutex
	repo UserRepository
}

// New returns a Store backed by repo.
func New(repo UserRepository) *Store {
	return &Store{repo: repo}
}

// ValidatePasswordStrength returns an error if password doesn't meet the minimum
// requirements: 12+ characters, at least one uppercase letter, one digit, and
// one special character.
func ValidatePasswordStrength(password string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	var hasUpper, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsDigit(c):
			hasDigit = true
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	return nil
}

// IsEmpty returns true when no users have been configured yet. A repository
// read error is treated as non-empty so a broken store never re-opens setup.
func (s *Store) IsEmpty() bool {
	users, err := s.repo.ListUsers()
	if err != nil {
		slog.Error("Failed to list users", "error", err, "component", "Auth")
		return false
	}
	return len(users) == 0
}

// Validate returns true when the username exists and password matches the stored hash.
func (s *Store) Validate(username, password string) bool {
	u, err := s.repo.GetUser(username)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			slog.Error("Failed to look up user", "error", err, "component", "Auth")
		}
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

// GetDisplayName returns the display name for the given username, or the username
// itself if no display name is set.
func (s *Store) GetDisplayName(username string) string {
	u, err := s.repo.GetUser(username)
	if err != nil || u.DisplayName == "" {
		return username
	}
	return u.DisplayName
}

// List returns a hash-free snapshot of all users.
func (s *Store) List() []UserInfo {
	users, err := s.repo.ListUsers()
	if err != nil {
		slog.Error("Failed to list users", "error", err, "component", "Auth")
		return []UserInfo{}
	}
	out := make([]UserInfo, len(users))
	for i, u := range users {
		out[i] = UserInfo{Username: u.Username, DisplayName: u.DisplayName}
	}
	return out
}

// SetUser creates or updates a user with the given username, display name, and password.
func (s *Store) SetUser(username, displayName, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, err := s.repo.GetUser(username)
	switch {
	case errors.Is(err, ErrNotFound):
		u = User{Username: username}
	case err != nil:
		return err
	}
	u.PasswordHash = string(hash)
	if displayName != "" {
		u.DisplayName = displayName
	}
	return s.repo.PutUser(u)
}

// UpdateProfile updates the display name for an existing user.
// newDisplayName may be empty to keep the current value.
func (s *Store) UpdateProfile(username, newDisplayName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, err := s.repo.GetUser(username)
	if errors.Is(err, ErrNotFound) {
		return errors.New("user not found")
	}
	if err != nil {
		return err
	}
	if newDisplayName == "" {
		return nil
	}
	u.DisplayName = newDisplayName
	return s.repo.PutUser(u)
}

// ChangePassword verifies currentPassword then replaces the hash with newPassword.
// Returns an error if currentPassword is wrong or newPassword fails strength checks.
func (s *Store) ChangePassword(username, currentPassword, newPassword string) error {
	if !s.Validate(username, currentPassword) {
		return errors.New("current password is incorrect")
	}
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}
	return s.SetUser(username, "", newPassword)
}
