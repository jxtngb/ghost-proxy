package client

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/jxtngb/ghost-proxy/pkg/crypto"
	"github.com/jxtngb/ghost-proxy/pkg/frame"
	"github.com/jxtngb/ghost-proxy/pkg/transport"
)

type Client struct {
	ServerAddress string
	ServerName    string
	PSK           []byte
}

func New(serverAddress, serverName string, psk []byte) *Client {
	return &Client{
		ServerAddress: serverAddress,
		ServerName:    serverName,
		PSK:           psk,
	}
}

func FromEnvironment(serverAddress, serverName string) (*Client, error) {
	value := os.Getenv("GHOST_PSK")
	if value == "" {
		return nil, fmt.Errorf("GHOST_PSK is not set")
	}

	psk, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode GHOST_PSK: %w", err)
	}

	if len(psk) == 0 {
		return nil, fmt.Errorf("GHOST_PSK is empty")
	}

	return New(serverAddress, serverName, psk), nil
}

func (c *Client) Dial(address string) (net.Conn, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if len(c.PSK) == 0 {
		return nil, fmt.Errorf("PSK is empty")
	}

	raw, err := net.DialTimeout("tcp", c.ServerAddress, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to Ghost server: %w", err)
	}

	tlsConn, err := transport.DialUTLS(raw, c.ServerName)
	if err != nil {
		return nil, err
	}

	exporter, err := transport.ExportKeyingMaterial(tlsConn)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}

	dataKey, authKey, err := crypto.DeriveSessionKeys(c.PSK, exporter)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("derive session keys: %w", err)
	}

	if err := authenticate(tlsConn, dataKey, authKey); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	channel, err := transport.NewDataChannel(dataKey)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}

	return &DataConn{
		conn:    tlsConn,
		channel: channel,
		target:  address,
	}, nil
}

func authenticate(conn net.Conn, dataKey, authKey []byte) error {
	f, err := frame.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read authentication challenge: %w", err)
	}

	if f.Type != frame.TypeAuthChallenge {
		return fmt.Errorf("unexpected authentication frame: 0x%02x", f.Type)
	}

	if len(f.Ciphertext) != crypto.ChallengeSize {
		return fmt.Errorf(
			"invalid authentication challenge length: %d",
			len(f.Ciphertext),
		)
	}

	response, err := crypto.ComputeResponse(authKey, f.Ciphertext)
	if err != nil {
		return fmt.Errorf("compute authentication response: %w", err)
	}

	aead, err := crypto.NewAEAD(dataKey)
	if err != nil {
		return fmt.Errorf("create authentication AEAD: %w", err)
	}

	nonce, err := crypto.RandomNonce()
	if err != nil {
		return fmt.Errorf("generate authentication nonce: %w", err)
	}

	ciphertext, err := aead.Seal(
		nonce,
		response,
		[]byte{frame.TypeAuthResponse},
	)
	if err != nil {
		return fmt.Errorf("encrypt authentication response: %w", err)
	}

	var nonceArray [frame.NonceSize]byte
	copy(nonceArray[:], nonce)

	return frame.WriteFrame(conn, &frame.Frame{
		Type:       frame.TypeAuthResponse,
		Nonce:      nonceArray,
		Ciphertext: ciphertext,
	})
}

type DataConn struct {
	conn    net.Conn
	channel *transport.DataChannel
	target  string

	readMu  sync.Mutex
	writeMu sync.Mutex

	readBuffer []byte
}

func (c *DataConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	for len(c.readBuffer) == 0 {
		payload, err := c.channel.ReadDataFrame(c.conn)
		if err != nil {
			return 0, err
		}

		c.readBuffer = payload
	}

	n := copy(p, c.readBuffer)
	c.readBuffer = c.readBuffer[n:]

	return n, nil
}

func (c *DataConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	const maxChunk = 32 * 1024

	total := 0

	for len(p) > 0 {
		chunk := p
		if len(chunk) > maxChunk {
			chunk = p[:maxChunk]
		}

		if err := c.channel.WriteDataFrame(c.conn, chunk); err != nil {
			return total, err
		}

		total += len(chunk)
		p = p[len(chunk):]
	}

	return total, nil
}

func (c *DataConn) Close() error {
	return c.conn.Close()
}

func (c *DataConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *DataConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *DataConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *DataConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *DataConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
