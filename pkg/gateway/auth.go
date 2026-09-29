package gateway

import (
	"errors"
	"fmt"

	ghostcrypto "github.com/jxtngb/ghost-proxy/pkg/crypto"
)

// AuthSession contains the cryptographic state for one authenticated
// gateway connection.
//
// The PSK and TLS exporter material are supplied by the connection layer.
// TLS integration itself is intentionally kept outside this package.
type AuthSession struct {
	dataKey []byte
	authKey []byte
}

// NewAuthSession derives the per-session data and authentication keys
// from the shared PSK and TLS exporter material.
func NewAuthSession(psk, exporterMaterial []byte) (*AuthSession, error) {
	if len(psk) == 0 {
		return nil, errors.New("gateway: PSK must not be empty")
	}

	dataKey, authKey, err := ghostcrypto.DeriveSessionKeys(psk, exporterMaterial)
	if err != nil {
		return nil, fmt.Errorf("gateway: derive session keys: %w", err)
	}

	return &AuthSession{
		dataKey: dataKey,
		authKey: authKey,
	}, nil
}

// DataKey returns the session key used for data encryption.
func (s *AuthSession) DataKey() []byte {
	if s == nil {
		return nil
	}

	return append([]byte(nil), s.dataKey...)
}

// Challenge creates a fresh authentication challenge.
func (s *AuthSession) Challenge() ([]byte, error) {
	if s == nil {
		return nil, errors.New("gateway: nil authentication session")
	}

	return ghostcrypto.GenerateChallenge()
}

// VerifyResponse validates a client's HMAC challenge response.
func (s *AuthSession) VerifyResponse(challenge, response []byte) (bool, error) {
	if s == nil {
		return false, errors.New("gateway: nil authentication session")
	}

	return ghostcrypto.VerifyResponse(s.authKey, challenge, response)
}
