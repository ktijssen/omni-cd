package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// Store is a thread-safe, file-backed list of users with bcrypt passwords.
type Store struct {
	mu    sync.RWMutex
	users []User
	path  string
}

// New loads (or creates) a Store backed by the file at path.
// Returns an empty store when the file does not yet exist.
func New(path string) (*Store, error) {
	s := &Store{path: path}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.users); err != nil {
		return nil, err
	}
	return s, nil
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

// IsEmpty returns true when no users have been configured yet.
func (s *Store) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users) == 0
}

// Validate returns true when the username exists and password matches the stored hash.
func (s *Store) Validate(username, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
		}
	}
	return false
}

// GetDisplayName returns the display name for the given username, or the username
// itself if no display name is set.
func (s *Store) GetDisplayName(username string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			if u.DisplayName != "" {
				return u.DisplayName
			}
			return username
		}
	}
	return username
}

// List returns a hash-free snapshot of all users.
func (s *Store) List() []UserInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserInfo, len(s.users))
	for i, u := range s.users {
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
	for i, u := range s.users {
		if u.Username == username {
			s.users[i].PasswordHash = string(hash)
			if displayName != "" {
				s.users[i].DisplayName = displayName
			}
			return s.save()
		}
	}
	s.users = append(s.users, User{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: string(hash),
	})
	return s.save()
}

// UpdateProfile updates the display name for an existing user.
// newDisplayName may be empty to keep the current value.
func (s *Store) UpdateProfile(username, newDisplayName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.users {
		if u.Username == username {
			if newDisplayName != "" {
				s.users[i].DisplayName = newDisplayName
			}
			return s.save()
		}
	}
	return errors.New("user not found")
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

// save writes the current user list to disk. Must be called with s.mu held (write).
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s.users)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
