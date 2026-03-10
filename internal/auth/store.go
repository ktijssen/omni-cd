package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// User holds a single account's credentials.
// The Username field is kept for reading legacy JSON files only; new records
// use Email + DisplayName. Migration runs automatically in New().
type User struct {
	// Legacy field — only populated when reading old data.
	Username string `json:"username,omitempty"`

	Email        string    `json:"email,omitempty"`
	DisplayName  string    `json:"displayName,omitempty"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

// UserInfo is a safe, hash-free view of a User for API responses.
type UserInfo struct {
	Email       string `json:"email"`
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
// Any legacy records that have Username but no Email are migrated in-place
// and the file is re-saved with the new schema.
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

	// Migration: Username-only records → Email + DisplayName.
	migrated := false
	for i, u := range s.users {
		if u.Email == "" && u.Username != "" {
			s.users[i].Email = u.Username
			s.users[i].DisplayName = u.Username
			s.users[i].Username = ""
			migrated = true
		}
	}
	if migrated {
		if err := s.save(); err != nil {
			return nil, err
		}
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

// Validate returns true when the email exists and password matches the stored hash.
func (s *Store) Validate(email, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
		}
	}
	return false
}

// GetDisplayName returns the display name for the given email, or the email
// itself if no display name is set.
func (s *Store) GetDisplayName(email string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			if u.DisplayName != "" {
				return u.DisplayName
			}
			return email
		}
	}
	return email
}

// List returns a hash-free snapshot of all users.
func (s *Store) List() []UserInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserInfo, len(s.users))
	for i, u := range s.users {
		out[i] = UserInfo{Email: u.Email, DisplayName: u.DisplayName}
	}
	return out
}

// SetUser creates or updates a user with the given email, display name, and
// password. Use this for initial setup and admin bootstrap.
func (s *Store) SetUser(email, displayName, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.users {
		if u.Email == email {
			s.users[i].PasswordHash = string(hash)
			if displayName != "" {
				s.users[i].DisplayName = displayName
			}
			return s.save()
		}
	}
	s.users = append(s.users, User{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	})
	return s.save()
}

// SetPassword updates the password for an existing user identified by email.
// Kept for backward-compatible use (ADMIN_PASSWORD env var bootstrap).
func (s *Store) SetPassword(email, password string) error {
	return s.SetUser(email, "", password)
}

// UpdateProfile updates the email and/or display name for an existing user.
// newEmail may be empty to keep the current email. newDisplayName may be empty
// to keep the current display name. Returns the (possibly unchanged) email so
// the caller can update the session.
func (s *Store) UpdateProfile(currentEmail, newEmail, newDisplayName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.users {
		if u.Email == currentEmail {
			if newEmail != "" {
				s.users[i].Email = newEmail
			}
			if newDisplayName != "" {
				s.users[i].DisplayName = newDisplayName
			}
			if err := s.save(); err != nil {
				return currentEmail, err
			}
			return s.users[i].Email, nil
		}
	}
	return currentEmail, errors.New("user not found")
}

// ChangePassword verifies currentPassword then replaces the hash with newPassword.
// Returns an error if currentPassword is wrong or newPassword fails strength checks.
func (s *Store) ChangePassword(email, currentPassword, newPassword string) error {
	if !s.Validate(email, currentPassword) {
		return errors.New("current password is incorrect")
	}
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return err
	}
	return s.SetPassword(email, newPassword)
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
