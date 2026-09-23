package gateway

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jxtngb/ghost-proxy/pkg/crypto"
	"github.com/jxtngb/ghost-proxy/pkg/frame"
)

// ErrAuthFailed is returned for every authentication failure. Callers
// (later: the Nginx fallback) should not need to know why it failed.
var ErrAuthFailed = errors.New("gateway: authentication failed")

// authTimeout bounds the whole handshake so a silent peer cannot hold
// a connection open forever.
var authTimeout = 10 * time.Second

// Authenticate runs the server side of the challenge-response handshake
// over conn:
//
//	server -> client: TypeAuthChallenge, 32-byte challenge
//	client -> server: TypeAuthResponse, AEAD(dataKey, HMAC(authKey, challenge))
//
// It returns nil only if the response verifies. TLS is not involved here;
// the caller supplies the session already derived from PSK + exporter.
func (s *AuthSession) Authenticate(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(authTimeout)); err != nil {
		return fmt.Errorf("gateway: set deadline: %w", err)
	}
	defer conn.SetDeadline(time.Time{})

	challenge, err := s.Challenge()
	if err != nil {
		return fmt.Errorf("gateway: generate challenge: %w", err)
	}

	nc, err := frame.NewNonceCounter()
	if err != nil {
		return err
	}
	if err := frame.WriteFrame(conn, &frame.Frame{
		Type:       frame.TypeAuthChallenge,
		Nonce:      nc.Next(),
		Ciphertext: challenge,
	}); err != nil {
		return fmt.Errorf("gateway: send challenge: %w", err)
	}

	f, err := frame.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("%w: read response: %v", ErrAuthFailed, err)
	}
	if f.Type != frame.TypeAuthResponse {
		return ErrAuthFailed
	}

	aead, err := crypto.NewAEAD(s.DataKey())
	if err != nil {
		return fmt.Errorf("gateway: init aead: %w", err)
	}
	response, err := aead.Open(f.Nonce[:], f.Ciphertext, []byte{f.Type})
	if err != nil {
		return ErrAuthFailed
	}

	ok, err := s.VerifyResponse(challenge, response)
	if err != nil || !ok {
		return ErrAuthFailed
	}
	return nil
}
