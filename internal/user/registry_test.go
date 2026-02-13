package user

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryManager_LoadOrCreate(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)

	// Load or create should create new registry
	reg, err := rm.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	if reg.Version != CurrentRegistryVersion {
		t.Errorf("version = %d, want %d", reg.Version, CurrentRegistryVersion)
	}
	if len(reg.Users) != 0 {
		t.Errorf("users = %d, want 0", len(reg.Users))
	}

	// Verify file was created
	path := RegistryPath(townRoot)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("registry file not created")
	}

	// Loading again should return same content
	reg2, err := rm.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg2.Version != CurrentRegistryVersion {
		t.Errorf("second load version = %d, want %d", reg2.Version, CurrentRegistryVersion)
	}
}

func TestRegistryManager_Add(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)

	// Add first user
	err := rm.Add(User{
		Username: "alice",
		Name:     "Alice Smith",
		Email:    "alice@example.com",
		Source:   SourceManual,
	})
	if err != nil {
		t.Fatalf("Add alice: %v", err)
	}

	// Verify user exists
	u, err := rm.Get("alice")
	if err != nil {
		t.Fatalf("Get alice: %v", err)
	}
	if u.Name != "Alice Smith" {
		t.Errorf("name = %q, want %q", u.Name, "Alice Smith")
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", u.Email, "alice@example.com")
	}
	if u.Added.IsZero() {
		t.Error("added timestamp should be set")
	}

	// Add second user
	err = rm.Add(User{
		Username: "bob",
		Name:     "Bob Jones",
		Source:   SourceGitConfig,
	})
	if err != nil {
		t.Fatalf("Add bob: %v", err)
	}

	// List should return both
	users, err := rm.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %d, want 2", len(users))
	}
}

func TestRegistryManager_AddDuplicate(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)

	err := rm.Add(User{Username: "alice", Name: "Alice", Source: SourceManual})
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}

	err = rm.Add(User{Username: "alice", Name: "Different Alice", Source: SourceManual})
	if !errors.Is(err, ErrUserExists) {
		t.Errorf("expected ErrUserExists, got: %v", err)
	}
}

func TestRegistryManager_AddEmptyUsername(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)

	err := rm.Add(User{Username: "", Name: "No Username", Source: SourceManual})
	if !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("expected ErrInvalidUsername, got: %v", err)
	}
}

func TestRegistryManager_Get(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)
	rm.Add(User{Username: "alice", Name: "Alice", Source: SourceManual})

	// Existing user
	u, err := rm.Get("alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username = %q, want %q", u.Username, "alice")
	}

	// Non-existent user
	_, err = rm.Get("nobody")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestRegistryManager_Remove(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)
	rm.Add(User{Username: "alice", Name: "Alice", Source: SourceManual})
	rm.Add(User{Username: "bob", Name: "Bob", Source: SourceManual})

	// Remove alice
	err := rm.Remove("alice")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify alice is gone
	_, err = rm.Get("alice")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("alice should be gone, got: %v", err)
	}

	// Bob should remain
	u, err := rm.Get("bob")
	if err != nil {
		t.Fatalf("Get bob: %v", err)
	}
	if u.Username != "bob" {
		t.Errorf("username = %q, want %q", u.Username, "bob")
	}

	// Remove non-existent
	err = rm.Remove("nobody")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestRegistryManager_Exists(t *testing.T) {
	townRoot := t.TempDir()
	os.MkdirAll(filepath.Join(townRoot, "mayor"), 0755)

	rm := NewRegistryManager(townRoot)
	rm.Add(User{Username: "alice", Name: "Alice", Source: SourceManual})

	exists, err := rm.Exists("alice")
	if err != nil {
		t.Fatalf("Exists alice: %v", err)
	}
	if !exists {
		t.Error("alice should exist")
	}

	exists, err = rm.Exists("nobody")
	if err != nil {
		t.Fatalf("Exists nobody: %v", err)
	}
	if exists {
		t.Error("nobody should not exist")
	}
}

func TestRegistryManager_LoadNoFile(t *testing.T) {
	townRoot := t.TempDir()
	rm := NewRegistryManager(townRoot)

	_, err := rm.Load()
	if !errors.Is(err, ErrRegistryNotFound) {
		t.Errorf("expected ErrRegistryNotFound, got: %v", err)
	}
}

func TestRegistryManager_ListNoFile(t *testing.T) {
	townRoot := t.TempDir()
	rm := NewRegistryManager(townRoot)

	users, err := rm.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if users != nil {
		t.Errorf("expected nil users, got: %v", users)
	}
}

func TestRegistryPath(t *testing.T) {
	got := RegistryPath("/home/user/gt")
	want := "/home/user/gt/mayor/users.json"
	if got != want {
		t.Errorf("RegistryPath = %q, want %q", got, want)
	}
}
