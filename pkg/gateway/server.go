package gateway

import (
	"errors"
	"net"
	"time"

	"github.com/jxtngb/ghost-proxy/pkg/logger"
)

// ExporterFunc returns the TLS exporter material for a connection.
// Day 4 replaces the stub with real TLS exporter extraction.
type ExporterFunc func(conn net.Conn) ([]byte, error)

// Server accepts connections and authenticates each one.
type Server struct {
	// PSK is the pre-shared key. Never logged.
	PSK []byte
	// Exporter supplies per-connection TLS exporter material.
	Exporter ExporterFunc
	// OnAuthenticated takes ownership of conn after a successful
	// handshake (tunnel handling will live here). If nil, conn is closed.
	OnAuthenticated func(conn net.Conn, s *AuthSession)
}

// Serve accepts connections on ln until ln is closed. Each connection
// is handled in its own goroutine.
func (srv *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			logger.Warn("accept failed", "err", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		go srv.handle(conn)
	}
}

func (srv *Server) handle(conn net.Conn) {
	remote := conn.RemoteAddr().String()

	exporter, err := srv.Exporter(conn)
	if err != nil {
		logger.Warn("exporter failed", "remote", remote, "err", err)
		conn.Close()
		return
	}

	session, err := NewAuthSession(srv.PSK, exporter)
	if err != nil {
		logger.Error("session setup failed", "remote", remote, "err", err)
		conn.Close()
		return
	}

	if err := session.Authenticate(conn); err != nil {
		// Later (Day 7): hand this connection to the Nginx fallback.
		logger.Warn("authentication failed", "remote", remote, "err", err)
		conn.Close()
		return
	}

	logger.Info("client authenticated", "remote", remote)
	if srv.OnAuthenticated == nil {
		conn.Close()
		return
	}
	srv.OnAuthenticated(conn, session)
}
