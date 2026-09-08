package socks5

import (
	"fmt"
	"net"
)

func connectToTarget(request *Request) (net.Conn, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Command != cmdConnect {
		return nil, fmt.Errorf("unsupported command: 0x%02x", request.Command)
	}

	conn, err := net.Dial("tcp", request.Address)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", request.Address, err)
	}

	return conn, nil
}
