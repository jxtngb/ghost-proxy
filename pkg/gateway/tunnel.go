package gateway

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jxtngb/ghost-proxy/pkg/crypto"
	"github.com/jxtngb/ghost-proxy/pkg/frame"
	"github.com/jxtngb/ghost-proxy/pkg/padding"
)

const DestinationDialTimeout = 10 * time.Second

const maxTargetAddressLen = 1024

// ServeTunnel receives the encrypted destination address from the client,
// connects to that destination, and forwards data in both directions.
func ServeTunnel(conn net.Conn, session *AuthSession) error {
	if conn == nil {
		return fmt.Errorf("gateway: connection is nil")
	}
	if session == nil {
		return fmt.Errorf("gateway: auth session is nil")
	}

	aead, err := crypto.NewAEAD(session.DataKey())
	if err != nil {
		return fmt.Errorf("gateway: create data AEAD: %w", err)
	}

	target, err := readConnectFrame(conn, aead)
	if err != nil {
		return fmt.Errorf("gateway: read connect frame: %w", err)
	}

	destination, err := net.DialTimeout(
		"tcp",
		target,
		DestinationDialTimeout,
	)
	if err != nil {
		return fmt.Errorf("gateway: connect to %q: %w", target, err)
	}
	defer destination.Close()

	errCh := make(chan error, 2)

	go func() {
		errCh <- forwardTunnelToDestination(conn, destination, aead)
	}()

	go func() {
		errCh <- forwardDestinationToTunnel(destination, conn, aead)
	}()

	return <-errCh
}

func readConnectFrame(r io.Reader, aead *crypto.AEAD) (string, error) {
	in, err := frame.ReadFrame(r)
	if err != nil {
		return "", fmt.Errorf("read frame: %w", err)
	}

	if in.Type != frame.TypeConnOpen {
		return "", fmt.Errorf(
			"unexpected frame type 0x%02x",
			in.Type,
		)
	}

	opened, err := aead.Open(
		in.Nonce[:],
		in.Ciphertext,
		[]byte{frame.TypeConnOpen},
	)
	if err != nil {
		return "", fmt.Errorf(
			"decrypt connect frame: %w",
			err,
		)
	}

	envelope, err := padding.Unpad(opened)
	if err != nil {
		return "", fmt.Errorf(
			"unpad connect frame: %w",
			err,
		)
	}

	payload, err := frame.DecodePayload(envelope)
	if err != nil {
		return "", fmt.Errorf(
			"decode connect frame: %w",
			err,
		)
	}

	target := string(payload)

	if err := validateTunnelTarget(target); err != nil {
		return "", err
	}

	return target, nil
}
func validateTunnelTarget(target string) error {
	if target == "" {
		return fmt.Errorf("gateway: target address is empty")
	}

	if len(target) > maxTargetAddressLen {
		return fmt.Errorf(
			"gateway: target address exceeds %d bytes",
			maxTargetAddressLen,
		)
	}

	host, portString, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf(
			"gateway: invalid target address %q: %w",
			target,
			err,
		)
	}

	if strings.TrimSpace(host) == "" {
		return fmt.Errorf(
			"gateway: target host is empty",
		)
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return fmt.Errorf(
			"gateway: invalid target port %q",
			portString,
		)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf(
			"gateway: target port out of range: %d",
			port,
		)
	}

	return nil
}

func forwardTunnelToDestination(
	tunnel net.Conn,
	destination net.Conn,
	aead *crypto.AEAD,
) error {
	for {
		in, err := frame.ReadFrame(tunnel)
		if err != nil {
			return err
		}

		if in.Type != frame.TypeDataPayload {
			return fmt.Errorf(
				"unexpected tunnel frame type 0x%02x",
				in.Type,
			)
		}

		opened, err := aead.Open(
			in.Nonce[:],
			in.Ciphertext,
			[]byte{frame.TypeDataPayload},
		)
		if err != nil {
			return fmt.Errorf(
				"decrypt data frame: %w",
				err,
			)
		}

		envelope, err := padding.Unpad(opened)
		if err != nil {
			return fmt.Errorf(
				"unpad data frame: %w",
				err,
			)
		}

		payload, err := frame.DecodePayload(envelope)
		if err != nil {
			return fmt.Errorf(
				"decode data frame: %w",
				err,
			)
		}

		if err := writeAll(destination, payload); err != nil {
			return fmt.Errorf(
				"write destination: %w",
				err,
			)
		}
	}
}

func forwardDestinationToTunnel(
	destination net.Conn,
	tunnel net.Conn,
	aead *crypto.AEAD,
) error {
	nonceCounter, err := frame.NewNonceCounter()
	if err != nil {
		return fmt.Errorf(
			"create nonce counter: %w",
			err,
		)
	}

	buffer := make([]byte, 32*1024)

	for {
		n, err := destination.Read(buffer)

		if n > 0 {
			if writeErr := writeDataFrame(
				tunnel,
				aead,
				nonceCounter,
				buffer[:n],
			); writeErr != nil {
				return writeErr
			}
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}

			return fmt.Errorf(
				"read destination: %w",
				err,
			)
		}
	}
}

func writeDataFrame(
	w io.Writer,
	aead *crypto.AEAD,
	nonceCounter *frame.NonceCounter,
	payload []byte,
) error {
	envelope, err := frame.EncodePayload(payload)
	if err != nil {
		return fmt.Errorf(
			"encode data payload: %w",
			err,
		)
	}

	padded, err := padding.Pad(
		envelope,
		aead.Overhead(),
	)
	if err != nil {
		return fmt.Errorf(
			"pad data payload: %w",
			err,
		)
	}

	nonce := nonceCounter.Next()

	ciphertext, err := aead.Seal(
		nonce[:],
		padded,
		[]byte{frame.TypeDataPayload},
	)
	if err != nil {
		return fmt.Errorf(
			"encrypt data payload: %w",
			err,
		)
	}

	out := &frame.Frame{
		Type:       frame.TypeDataPayload,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	return frame.WriteFrame(w, out)
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrUnexpectedEOF
		}

		data = data[n:]
	}

	return nil
}
