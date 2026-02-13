package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	// ErrUserNotFound indicates the requested user does not exist.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserExists indicates a user with that username already exists.
	ErrUserExists = errors.New("user already exists")

	// ErrInvalidUsername indicates the username is empty or invalid.
	ErrInvalidUsername = errors.New("invalid username")

	// ErrRegistryNotFound indicates the registry file does not exist.
	ErrRegistryNotFound = errors.New("user registry not found")
)

// RegistryPath returns the standard path for the user registry in a town.
func RegistryPath(townRoot string) string {
	return filepath.Join(townRoot, "mayor", "users.json")
}

// RegistryManager provides thread-safe user registry operations.
type RegistryManager struct {
	mu       sync.Mutex
	townRoot string
}

// NewRegistryManager creates a new RegistryManager for the given town root.
func NewRegistryManager(townRoot string) *RegistryManager {
	return &RegistryManager{townRoot: townRoot}
}

// Load reads the user registry from disk.
func (rm *RegistryManager) Load() (*Registry, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	return rm.loadLocked()
}

// loadLocked reads the registry without acquiring the lock (caller must hold it).
func (rm *RegistryManager) loadLocked() (*Registry, error) {
	path := RegistryPath(rm.townRoot)
	data, err := os.ReadFile(path) //nolint:gosec // G304: path from trusted town root
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrRegistryNotFound, path)
		}
		return nil, fmt.Errorf("reading user registry: %w", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing user registry: %w", err)
	}

	return &reg, nil
}

// Save writes the user registry to disk.
func (rm *RegistryManager) Save(reg *Registry) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	return rm.saveLocked(reg)
}

// saveLocked writes the registry without acquiring the lock (caller must hold it).
func (rm *RegistryManager) saveLocked(reg *Registry) error {
	path := RegistryPath(rm.townRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding user registry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil { //nolint:gosec // G306: registry is not secret
		return fmt.Errorf("writing user registry: %w", err)
	}

	return nil
}

// LoadOrCreate loads existing registry or creates a new empty one.
func (rm *RegistryManager) LoadOrCreate() (*Registry, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	reg, err := rm.loadLocked()
	if err == nil {
		return reg, nil
	}

	if !errors.Is(err, ErrRegistryNotFound) {
		return nil, err
	}

	// Create new empty registry
	reg = &Registry{
		Version: CurrentRegistryVersion,
		Users:   []User{},
	}

	if err := rm.saveLocked(reg); err != nil {
		return nil, fmt.Errorf("creating user registry: %w", err)
	}

	return reg, nil
}

// Add adds a new user to the registry. Returns ErrUserExists if username is taken.
func (rm *RegistryManager) Add(u User) error {
	if u.Username == "" {
		return ErrInvalidUsername
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	reg, err := rm.loadLocked()
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			reg = &Registry{
				Version: CurrentRegistryVersion,
				Users:   []User{},
			}
		} else {
			return err
		}
	}

	// Check for duplicate
	for _, existing := range reg.Users {
		if existing.Username == u.Username {
			return fmt.Errorf("%w: %s", ErrUserExists, u.Username)
		}
	}

	if u.Added.IsZero() {
		u.Added = time.Now().UTC()
	}

	reg.Users = append(reg.Users, u)
	return rm.saveLocked(reg)
}

// Get returns the user with the given username.
func (rm *RegistryManager) Get(username string) (*User, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	reg, err := rm.loadLocked()
	if err != nil {
		return nil, err
	}

	for i := range reg.Users {
		if reg.Users[i].Username == username {
			return &reg.Users[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrUserNotFound, username)
}

// List returns all users in the registry.
func (rm *RegistryManager) List() ([]User, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	reg, err := rm.loadLocked()
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return reg.Users, nil
}

// Remove removes a user from the registry by username.
func (rm *RegistryManager) Remove(username string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	reg, err := rm.loadLocked()
	if err != nil {
		return err
	}

	for i, u := range reg.Users {
		if u.Username == username {
			reg.Users = append(reg.Users[:i], reg.Users[i+1:]...)
			return rm.saveLocked(reg)
		}
	}

	return fmt.Errorf("%w: %s", ErrUserNotFound, username)
}

// Exists checks if a user with the given username exists.
func (rm *RegistryManager) Exists(username string) (bool, error) {
	_, err := rm.Get(username)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrUserNotFound) || errors.Is(err, ErrRegistryNotFound) {
		return false, nil
	}
	return false, err
}
