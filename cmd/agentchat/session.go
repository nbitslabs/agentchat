package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/nbitslabs/agentchat/internal/apiclient"
	"github.com/nbitslabs/agentchat/internal/credstore"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

func cmdCreateSession() {
	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !store.HasIdentity() {
		fmt.Fprintln(os.Stderr, "No identity found. Run 'agentchat auth generate' and 'agentchat register' first.")
		os.Exit(1)
	}

	if err := createAndSaveSession(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createAndSaveSession(store *credstore.Store) error {
	identity, err := store.LoadIdentity()
	if err != nil {
		return fmt.Errorf("loading identity: %w", err)
	}

	rootPrivKey, err := identity.GetSecretKey()
	if err != nil {
		return fmt.Errorf("decoding secret key: %w", err)
	}

	// Generate session key pair
	sessPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating session key: %w", err)
	}

	// Sign session public key with root private key
	signature := ed25519.Sign(rootPrivKey, sessPub)

	client := apiclient.New()
	resp, err := client.CreateSession(
		identity.AgentID,
		credstore.EncodeBase64(sessPub),
		credstore.EncodeBase64(signature),
	)
	if err != nil {
		return fmt.Errorf("API call: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("session creation failed: %s", resp.Error.Message)
	}

	var data struct {
		SessionToken string `json:"session_token"`
		ExpiresAt    string `json:"expires_at"`
	}
	json.Unmarshal(resp.Data, &data)

	err = store.SaveSession(&credstore.SessionInfo{
		SessionToken: data.SessionToken,
		ExpiresAt:    data.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	expires, _ := time.Parse(time.RFC3339, data.ExpiresAt)
	fmt.Println("Session created!")
	fmt.Printf("  Expires: %s (%s remaining)\n", data.ExpiresAt, time.Until(expires).Round(time.Second))

	return nil
}

func cmdClaimUsername(username string) {
	if !usernamePattern.MatchString(username) {
		fmt.Fprintln(os.Stderr, "Invalid username: must be 3-32 characters, alphanumeric, hyphens, or underscores")
		os.Exit(1)
	}

	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	token, err := ensureSession(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := apiclient.New()
	resp, err := client.ClaimUsername(token, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Username claim failed: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	fmt.Printf("Username '%s' claimed! Status: pending approval.\n", username)
}

// ensureSession checks if the current session is valid and refreshes if needed.
func ensureSession(store *credstore.Store) (string, error) {
	sess, err := store.LoadSession()
	if err != nil {
		// No session, create one
		if err := createAndSaveSession(store); err != nil {
			return "", err
		}
		sess, err = store.LoadSession()
		if err != nil {
			return "", err
		}
		return sess.SessionToken, nil
	}

	// Check if expiring within 15 minutes
	expires, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil || time.Until(expires) < 15*time.Minute {
		// Refresh session
		if err := createAndSaveSession(store); err != nil {
			return "", fmt.Errorf("session refresh failed: %w", err)
		}
		sess, err = store.LoadSession()
		if err != nil {
			return "", err
		}
	}

	return sess.SessionToken, nil
}
