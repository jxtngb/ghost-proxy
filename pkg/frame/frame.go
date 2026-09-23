// Package frame implements the Ghost Proxy framed wire protocol.
//
// Wire format (see project spec, "Framed Protocol Wire Format"):
//
//	+-----------+-------------+--------------+
//	| Type (1B) | Length (2B) | Nonce (12B)  |
//	+-----------+-------------+--------------+
//	| Ciphertext (Length bytes)              |
//	+-----------------------------------------+
//
// The ciphertext is produced by pkg/crypto (ChaCha20-Poly1305 AEAD) over a
// plaintext envelope built by EncodePayload. This package owns only the
// header and raw framing (read/write of opaque frames) - it never
// encrypts or decrypts. pkg/crypto seals into Frame.Ciphertext and opens
// it back out.
package frame

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame types, per the wire format specification.
const (
	TypeAuthChallenge byte = 0x01
	TypeAuthResponse  byte = 0x02
	TypeDataPayload   byte = 0x03
	TypeConnClose     byte = 0x04
)

// NonceSize is the ChaCha20-Poly1305 nonce length in bytes.
const NonceSize = 12

// HeaderSize is the fixed on-wire header size: Type(1) + Length(2) + Nonce(12).
const HeaderSize = 1 + 2 + NonceSize

// MaxCiphertextLen is the largest ciphertext length representable in the
// 2-byte Length field.
const MaxCiphertextLen = 1<<16 - 1

// Frame is a single unit of the Ghost Proxy wire protocol: a header plus
// an opaque ciphertext blob. Ciphertext comes from pkg/crypto's Seal and
// is handed back to its Open - this package never looks inside it.
type Frame struct {
	Type       byte
	Nonce      [NonceSize]byte
	Ciphertext []byte
}

// IsValidType reports whether t is one of the defined frame types.
func IsValidType(t byte) bool {
	switch t {
	case TypeAuthChallenge, TypeAuthResponse, TypeDataPayload, TypeConnClose:
		return true
	default:
		return false
	}
}

// WriteFrame encodes f and writes it to w as Type | Length | Nonce | Ciphertext.
func WriteFrame(w io.Writer, f *Frame) error {
	if f == nil {
		return fmt.Errorf("frame: cannot write nil frame")
	}

	if !IsValidType(f.Type) {
		return fmt.Errorf("frame: invalid frame type: 0x%02x", f.Type)
	}

	if len(f.Ciphertext) > MaxCiphertextLen {
		return fmt.Errorf(
			"frame: ciphertext too large: %d bytes (max %d)",
			len(f.Ciphertext), MaxCiphertextLen,
		)
	}

	header := make([]byte, HeaderSize)
	header[0] = f.Type
	binary.BigEndian.PutUint16(header[1:3], uint16(len(f.Ciphertext)))
	copy(header[3:3+NonceSize], f.Nonce[:])

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("frame: write header: %w", err)
	}

	if len(f.Ciphertext) > 0 {
		if _, err := w.Write(f.Ciphertext); err != nil {
			return fmt.Errorf("frame: write ciphertext: %w", err)
		}
	}

	return nil
}

// ReadFrame reads and decodes a single Frame from r. It performs no
// decryption - f.Ciphertext is the raw sealed blob for pkg/crypto to open.
func ReadFrame(r io.Reader) (*Frame, error) {
	header := make([]byte, HeaderSize)

	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("frame: read header: %w", err)
	}

	frameType := header[0]

	if !IsValidType(frameType) {
		return nil, fmt.Errorf("frame: invalid frame type: 0x%02x", frameType)
	}

	length := binary.BigEndian.Uint16(header[1:3])

	var nonce [NonceSize]byte
	copy(nonce[:], header[3:3+NonceSize])

	ciphertext := make([]byte, length)

	if length > 0 {
		if _, err := io.ReadFull(r, ciphertext); err != nil {
			return nil, fmt.Errorf("frame: read ciphertext: %w", err)
		}
	}

	return &Frame{
		Type:       frameType,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}
