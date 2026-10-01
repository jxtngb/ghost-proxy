package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	utls "github.com/refraction-networking/utls"

	"github.com/jxtngb/ghost-proxy/pkg/config"
	"github.com/jxtngb/ghost-proxy/pkg/gateway"
	"github.com/jxtngb/ghost-proxy/pkg/logger"
	"github.com/jxtngb/ghost-proxy/pkg/transport"
)

const minPSKLen = 16

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
	logger.SetLevel(cfg.LogLevel)
	if *listen != "" {
		cfg.ListenAddress = *listen
	}
	if cfg.ListenAddress == "" {
		return errors.New("no listen address configured")
	}
	pskHex := os.Getenv("GHOST_PSK")
	if pskHex == "" {
		return errors.New("GHOST_PSK must be set")
	}

	psk, err := hex.DecodeString(pskHex)
	if err != nil {
		return fmt.Errorf("decode GHOST_PSK: %w", err)
	}

	if len(psk) < 16 {
		return errors.New("GHOST_PSK must decode to at least 16 bytes")
	}

	tlsConfig, err := transport.TLSServerConfig(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return err
	}

	rawLn, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return err
	}

	ln := rawLn

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
		Prepare: func(raw net.Conn) (net.Conn, error) {
			_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
			tlsConn, err := transport.TLSServer(raw, tlsConfig)
			if err != nil {
				return nil, err
			}
			_ = tlsConn.SetDeadline(time.Time{})
			return tlsConn, nil
		},
		OnFallback: func(conn net.Conn, replay io.Reader) { proxyToNginx(conn, replay, cfg.FallbackAddress) },
		OnAuthenticated: func(conn net.Conn, session *gateway.AuthSession) {
			defer conn.Close()

			targetInfo := "authenticated connection"
			if remote := conn.RemoteAddr(); remote != nil {
				targetInfo += " remote=" + remote.String()
			}

			logger.Info(targetInfo)

			if err := gateway.ServeTunnelWithOptions(conn, session, gateway.TunnelOptions{AllowedDestinations: cfg.AllowedDestinations, PaddingEnabled: cfg.PaddingEnabled, JitterMS: cfg.JitterMS}); err != nil {
				target := conn.RemoteAddr()
				if target != nil {
					logger.Warn(
						"tunnel ended",
						"remote",
						target.String(),
						"error",
						err,
					)
				} else {
					logger.Warn("tunnel ended", "error", err)
				}
			}
		},
	}

	logger.Info("gateway listening", "addr", ln.Addr().String())
	return srv.Serve(ln)
}

func proxyToNginx(client net.Conn, replay io.Reader, addr string) {
	if addr == "" {
		_ = client.Close()
		return
	}
	upstream, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		_ = client.Close()
		return
	}
	defer upstream.Close()
	defer client.Close()
	done := make(chan struct{}, 2)
	go func() {
		_, copyErr := relayCopy(upstream, replay, client, 5*time.Minute)
		if copyErr != nil {
			_ = client.Close()
			_ = upstream.Close()
		}
		if c, ok := upstream.(*net.TCPConn); ok {
			_ = c.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, copyErr := relayCopy(client, upstream, upstream, 5*time.Minute)
		if copyErr != nil {
			_ = client.Close()
			_ = upstream.Close()
		}
		if c, ok := client.(*net.TCPConn); ok {
			_ = c.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
}

func relayCopy(dst net.Conn, src io.Reader, readConn net.Conn, idle time.Duration) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		_ = readConn.SetReadDeadline(time.Now().Add(idle))
		n, rerr := src.Read(buf)
		if n > 0 {
			_ = dst.SetWriteDeadline(time.Now().Add(idle))
			if werr := writeFallbackAll(dst, buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if rerr != nil {
			if rerr == io.EOF {
				return total, nil
			}
			return total, rerr
		}
	}
}

func writeFallbackAll(dst net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := dst.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
