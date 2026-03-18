package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nbitslabs/agentchat/internal/credstore"
	"github.com/nbitslabs/agentchat/internal/crypto"
)

func cmdAuthGenerate() {
	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if store.HasIdentity() {
		fmt.Print("An identity already exists. Overwrite? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key pair: %v\n", err)
		os.Exit(1)
	}

	agentID := crypto.DeriveAgentID(pub)

	identity := &credstore.Identity{
		AgentID:       agentID,
		RootPublicKey: encodeBase64(pub),
		RootSecretKey: encodeBase64(priv),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := store.SaveIdentity(identity); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving identity: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Identity generated successfully!")
	fmt.Printf("  Agent ID:    %s\n", agentID)
	fmt.Printf("  Public Key:  %s\n", identity.RootPublicKey)
	fmt.Printf("  Fingerprint: %s\n", computeFP(pub))
	fmt.Printf("  Stored at:   %s/identity.json\n", store.Dir())
}

func cmdAuthImport(keyFile string) {
	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if store.HasIdentity() {
		fmt.Print("An identity already exists. Overwrite? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	data, err := os.ReadFile(keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading key file: %v\n", err)
		os.Exit(1)
	}

	// Try to decode as base64
	decoded, err := decodeBase64Bytes(strings.TrimSpace(string(data)))
	if err != nil {
		// Try raw bytes
		decoded = data
	}

	if len(decoded) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "Error: invalid key size (%d bytes, expected %d)\n", len(decoded), ed25519.PrivateKeySize)
		os.Exit(1)
	}

	priv := ed25519.PrivateKey(decoded)
	pub := priv.Public().(ed25519.PublicKey)
	agentID := crypto.DeriveAgentID(pub)

	identity := &credstore.Identity{
		AgentID:       agentID,
		RootPublicKey: encodeBase64(pub),
		RootSecretKey: encodeBase64(priv),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := store.SaveIdentity(identity); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving identity: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Identity imported successfully!")
	fmt.Printf("  Agent ID:    %s\n", agentID)
	fmt.Printf("  Public Key:  %s\n", identity.RootPublicKey)
	fmt.Printf("  Fingerprint: %s\n", computeFP(pub))
}

func cmdAuthStatus() {
	store, err := credstore.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !store.HasIdentity() {
		fmt.Println("No identity configured.")
		fmt.Println("Run 'agentchat auth generate' to create one.")
		return
	}

	identity, err := store.LoadIdentity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading identity: %v\n", err)
		os.Exit(1)
	}

	pubBytes, _ := decodeBase64Bytes(identity.RootPublicKey)
	fmt.Println("Identity:")
	fmt.Printf("  Agent ID:    %s\n", identity.AgentID)
	fmt.Printf("  Public Key:  %s\n", identity.RootPublicKey)
	if len(pubBytes) > 0 {
		fmt.Printf("  Fingerprint: %s\n", computeFP(pubBytes))
	}
	fmt.Printf("  Created:     %s\n", identity.CreatedAt)

	fmt.Println()

	if store.IsSessionValid() {
		sess, _ := store.LoadSession()
		expires, _ := time.Parse(time.RFC3339, sess.ExpiresAt)
		fmt.Println("Session: active")
		fmt.Printf("  Expires: %s (%s remaining)\n", sess.ExpiresAt, time.Until(expires).Round(time.Second))
	} else {
		fmt.Println("Session: none (run 'agentchat register' and 'agentchat login' to authenticate)")
	}
}

func computeFP(pub []byte) string {
	hash := sha256.Sum256(pub)
	parts := make([]string, 16)
	for i := 0; i < 16; i++ {
		parts[i] = fmt.Sprintf("%02x", hash[i])
	}
	return strings.Join(parts, ":")
}

func encodeBase64(b []byte) string {
	return credstore.EncodeBase64(b)
}

func decodeBase64Bytes(s string) ([]byte, error) {
	return credstore.DecodeBase64(s)
}
