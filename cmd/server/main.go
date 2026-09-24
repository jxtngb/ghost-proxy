package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	utls "github.com/refraction-networking/utls"

	"github.com/jxtngb/ghost-proxy/pkg/config"
	"github.com/jxtngb/ghost-proxy/pkg/gateway"
	"github.com/jxtngb/ghost-proxy/pkg/logger"
	"github.com/jxtngb/ghost-proxy/pkg/transport"
)

const minPSKLen = 16

type tlsListener struct {
	net.Listener
	Config *utls.Config
}

func (l *tlsListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	tlsConn, err := transport.TLSServer(conn, l.Config)
	if err != nil {
		return nil, err
	}

	return tlsConn, nil
}

func exportServerKeyingMaterial(conn net.Conn) ([]byte, error) {
	tlsConn, ok := conn.(*utls.Conn)
	if !ok {
		return nil, fmt.Errorf(
			"ghost-proxy: expected TLS connection, got %T",
			conn,
		)
	}

	return transport.ExportServerKeyingMaterial(tlsConn)
}

func main() {
	if err := run(); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := flag.String("config", "configs/server.yaml", "path to server config")
	listen := flag.String("listen", "", "override listen address from config")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if *listen != "" {
		cfg.ListenAddress = *listen
	}
	if cfg.ListenAddress == "" {
		return errors.New("no listen address configured")
	}

	psk := os.Getenv("GHOST_PSK")
	if len(psk) < minPSKLen {
		return errors.New("GHOST_PSK must be set and at least 16 characters")
	}

	tlsConfig, err := transport.TLSServerConfig(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return err
	}

	rawLn, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return err
	}

	ln := &tlsListener{
		Listener: rawLn,
		Config:   tlsConfig,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		ln.Close()
	}()

	srv := &gateway.Server{
		PSK:      []byte(psk),
		Exporter: exportServerKeyingMaterial,
		OnAuthenticated: func(conn net.Conn, _ *gateway.AuthSession) {
			// Tunnel / TCP forwarding arrives on Day 5-7.
			logger.Info(
				"authenticated connection (no tunnel yet)",
				"remote",
				conn.RemoteAddr().String(),
			)
			conn.Close()
		},
	}

	logger.Info("gateway listening", "addr", ln.Addr().String())
	return srv.Serve(ln)
}
