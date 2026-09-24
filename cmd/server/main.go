package main

import (
	"context"
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jxtngb/ghost-proxy/pkg/config"
	"github.com/jxtngb/ghost-proxy/pkg/gateway"
	"github.com/jxtngb/ghost-proxy/pkg/logger"
)

const minPSKLen = 16

// stubExporter stands in for real TLS exporter material until Day 4.
// Client and gateway must use the same value for the handshake to pass.
func stubExporter(net.Conn) ([]byte, error) {
	return []byte("ghost-proxy-day3-placeholder-exporter"), nil
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

	ln, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return err
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
		Exporter: stubExporter,
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
