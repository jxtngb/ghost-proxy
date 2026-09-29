package transport

import (
	"fmt"
	"io"

	"github.com/jxtngb/ghost-proxy/pkg/crypto"
	"github.com/jxtngb/ghost-proxy/pkg/frame"
	"github.com/jxtngb/ghost-proxy/pkg/padding"
)

// DataChannel provides the Ghost Proxy data-frame
// encode/pad/encrypt/decrypt/unpad/decode pipeline.
type DataChannel struct {
	AEAD       *crypto.AEAD
	SendNonces *frame.NonceCounter
}

// NewDataChannel creates a data channel from a session key.
func NewDataChannel(key []byte) (*DataChannel, error) {
	aead, err := crypto.NewAEAD(key)
	if err != nil {
		return nil, fmt.Errorf("transport: create data AEAD: %w", err)
	}

	nonces, err := frame.NewNonceCounter()
	if err != nil {
		return nil, fmt.Errorf("transport: create nonce counter: %w", err)
	}

	return &DataChannel{
		AEAD:       aead,
		SendNonces: nonces,
	}, nil
}

// WriteDataFrame performs:
//
//	EncodePayload -> Pad -> Seal -> WriteFrame
func (d *DataChannel) WriteDataFrame(w io.Writer, payload []byte) error {
	if d == nil || d.AEAD == nil || d.SendNonces == nil {
		return fmt.Errorf("transport: data channel is not initialized")
	}

	if w == nil {
		return fmt.Errorf("transport: writer is nil")
	}

	envelope, err := frame.EncodePayload(payload)
	if err != nil {
		return fmt.Errorf("transport: encode payload: %w", err)
	}

	padded, err := padding.Pad(envelope, d.AEAD.Overhead())
	if err != nil {
		return fmt.Errorf("transport: pad payload: %w", err)
	}

	nonce := d.SendNonces.Next()

	ciphertext, err := d.AEAD.Seal(
		nonce[:],
		padded,
		[]byte{frame.TypeDataPayload},
	)
	if err != nil {
		return fmt.Errorf("transport: seal data frame: %w", err)
	}

	out := &frame.Frame{
		Type:       frame.TypeDataPayload,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	if err := frame.WriteFrame(w, out); err != nil {
		return fmt.Errorf("transport: write data frame: %w", err)
	}

	return nil
}

// ReadDataFrame performs:
//
//	ReadFrame -> Open -> Unpad -> DecodePayload
func (d *DataChannel) ReadDataFrame(r io.Reader) ([]byte, error) {
	if d == nil || d.AEAD == nil {
		return nil, fmt.Errorf("transport: data channel is not initialized")
	}

	if r == nil {
		return nil, fmt.Errorf("transport: reader is nil")
	}

	in, err := frame.ReadFrame(r)
	if err != nil {
		return nil, fmt.Errorf("transport: read data frame: %w", err)
	}

	if in.Type != frame.TypeDataPayload {
		return nil, fmt.Errorf(
			"transport: unexpected frame type: 0x%02x",
			in.Type,
		)
	}

	opened, err := d.AEAD.Open(
		in.Nonce[:],
		in.Ciphertext,
		[]byte{in.Type},
	)
	if err != nil {
		return nil, fmt.Errorf("transport: open data frame: %w", err)
	}

	envelope, err := padding.Unpad(opened)
	if err != nil {
		return nil, fmt.Errorf("transport: unpad data frame: %w", err)
	}

	payload, err := frame.DecodePayload(envelope)
	if err != nil {
		return nil, fmt.Errorf("transport: decode payload: %w", err)
	}

	return payload, nil
}
