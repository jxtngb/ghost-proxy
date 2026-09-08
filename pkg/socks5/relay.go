package socks5

import (
	"io"
	"net"
)

func relay(clientConn, targetConn net.Conn) error {
	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(targetConn, clientConn)
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(clientConn, targetConn)
		errCh <- err
	}()

	return <-errCh
}
