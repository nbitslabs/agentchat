package credstore

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	dirPerm  = 0700
	filePerm = 0600
)

// Identity holds the agent's cryptographic identity.
type Identity struct {
	AgentID       string `json:"agent_id"`
	RootPublicKey string `json:"root_public_key"`
	RootSecretKey string `json:"root_secret_key"`
	CreatedAt     string `json:"created_at"`
}

// SessionInfo holds cached session information.
type SessionInfo struct {
	SessionToken string `json:"session_token"`
	ExpiresAt    string `json:"expires_at"`
}

// Store manages the credential store at ~/.agentchat/.
type Store struct {
	dir string
}

// New creates a credential store. Uses AGENTCHAT_HOME env var or defaults to ~/.agentchat/.
func New() (*Store, error) {
	dir := os.Getenv("AGENTCHAT_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		dir = filepath.Join(home, ".agentchat")
	}
	return &Store{dir: dir}, nil
}

// Dir returns the credential store directory path.
func (s *Store) Dir() string {
	return s.dir
}

// EnsureDir creates the credential store directory with secure permissions.
func (s *Store) EnsureDir() error {
	info, err := os.Stat(s.dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(s.dir, dirPerm)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", s.dir)
	}
	// Check permissions
	if info.Mode().Perm() != dirPerm {
		fmt.Fprintf(os.Stderr, "WARNING: %s has permissions %o, expected %o\n", s.dir, info.Mode().Perm(), dirPerm)
	}
	return nil
}

// HasIdentity returns true if an identity file exists.
func (s *Store) HasIdentity() bool {
	_, err := os.Stat(filepath.Join(s.dir, "identity.json"))
	return err == nil
}

// SaveIdentity writes the identity to the credential store.
func (s *Store) SaveIdentity(id *Identity) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "identity.json"), data, filePerm)
}

// LoadIdentity reads the identity from the credential store.
func (s *Store) LoadIdentity() (*Identity, error) {
	path := filepath.Join(s.dir, "identity.json")

	// Check permissions
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != filePerm {
		fmt.Fprintf(os.Stderr, "WARNING: %s has permissions %o, expected %o\n", path, info.Mode().Perm(), filePerm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var id Identity
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}
	return &id, nil
}

// SaveSession writes session info to the credential store.
func (s *Store) SaveSession(sess *SessionInfo) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "session.json"), data, filePerm)
}

// LoadSession reads session info from the credential store.
func (s *Store) LoadSession() (*SessionInfo, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "session.json"))
	if err != nil {
		return nil, err
	}
	var sess SessionInfo
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// IsSessionValid checks if the stored session is still valid.
func (s *Store) IsSessionValid() bool {
	sess, err := s.LoadSession()
	if err != nil {
		return false
	}
	expires, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil {
		return false
	}
	return time.Now().Before(expires)
}

// GetSecretKey decodes the stored secret key.
func (id *Identity) GetSecretKey() (ed25519.PrivateKey, error) {
	decoded, err := DecodeBase64(id.RootSecretKey)
	if err != nil {
		return nil, fmt.Errorf("invalid secret key encoding: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid secret key size: expected %d, got %d", ed25519.PrivateKeySize, len(decoded))
	}
	return ed25519.PrivateKey(decoded), nil
}
