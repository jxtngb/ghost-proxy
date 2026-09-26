package socks5

import (
	"encoding/hex"
	"fmt"
	"io"
	"net"
)

const (
	socks5Version    = 0x05
	noAuthentication = 0x00
)

type DialFunc func(address string) (net.Conn, error)

type Server struct {
	Addr string
	Dial DialFunc
}

func (s *Server) Start() error {
	if s.Addr == "" {
		s.Addr = "127.0.0.1:1080"
	}

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.Addr, err)
	}
	defer listener.Close()

	fmt.Printf("SOCKS5 server listening on %s\n", s.Addr)

	dial := s.Dial
	if dial == nil {
		dial = func(address string) (net.Conn, error) {
			return net.Dial("tcp", address)
		}
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accept connection: %w", err)
		}

		go func(conn net.Conn) {
			defer conn.Close()

			if err := handleConnection(conn, dial); err != nil {
				fmt.Printf("SOCKS5 connection failed: %v\n", err)
			}
		}(conn)
	}
}

func handleConnection(conn net.Conn, dial DialFunc) error {
	if err := handleGreeting(conn); err != nil {
		return fmt.Errorf("greeting: %w", err)
	}

	request, err := readRequest(conn)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	targetConn, err := connectToTarget(request, dial)
	if err != nil {
		if replyErr := sendReply(
			conn,
			replyConnectionRefused,
			&net.TCPAddr{},
		); replyErr != nil {
			return fmt.Errorf(
				"connect failed: %w; send reply failed: %v",
				err,
				replyErr,
			)
		}

		return fmt.Errorf("connect to target: %w", err)
	}
	defer targetConn.Close()

	if err := sendReply(
		conn,
		replySucceeded,
		targetConn.LocalAddr(),
	); err != nil {
		return fmt.Errorf("send success reply: %w", err)
	}

	if err := relay(conn, targetConn); err != nil {
		return fmt.Errorf("relay: %w", err)
	}

	return nil
}

func handleGreeting(conn net.Conn) error {
	header := make([]byte, 2)

	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read greeting header: %w", err)
	}

	version := header[0]
	methodCount := int(header[1])

	if version != socks5Version {
		return fmt.Errorf("unsupported SOCKS version: 0x%02x", version)
	}

	if methodCount == 0 {
		return fmt.Errorf("client offered no authentication methods")
	}

	methods := make([]byte, methodCount)

	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("read authentication methods: %w", err)
	}

	fmt.Printf("SOCKS5 methods: %s\n", hex.EncodeToString(methods))

	for _, method := range methods {
		if method == noAuthentication {
			_, err := conn.Write([]byte{socks5Version, noAuthentication})
			if err != nil {
				return fmt.Errorf("send method selection: %w", err)
			}

			return nil
		}
	}

	_, err := conn.Write([]byte{socks5Version, 0xff})
	if err != nil {
		return fmt.Errorf("send no-acceptable-methods response: %w", err)
	}

	return fmt.Errorf("no supported authentication method")
}
