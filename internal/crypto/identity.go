package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
)

// DeriveAgentID derives an agent ID from an Ed25519 public key.
// Format: agnt_ + base58(SHA256(pubkey)[:24])
func DeriveAgentID(pubkey ed25519.PublicKey) string {
	hash := sha256.Sum256(pubkey)
	encoded := base58.Encode(hash[:24])
	return "agnt_" + encoded
}

// DecodePublicKey decodes a base64-encoded Ed25519 public key.
func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(decoded))
	}
	return ed25519.PublicKey(decoded), nil
}
