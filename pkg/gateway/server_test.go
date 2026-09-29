package gateway

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/jxtngb/ghost-proxy/pkg/crypto"
	"github.com/jxtngb/ghost-proxy/pkg/frame"
)

// hsClientAuth plays a complete client over TCP. It returns errors
// instead of calling t.Fatal so it is safe to use from goroutines.
func hsClientAuth(addr string, psk, exporter []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}

	f, err := frame.ReadFrame(conn)
	if err != nil {
		return err
	}
	dataKey, authKey, err := crypto.DeriveSessionKeys(psk, exporter)
	if err != nil {
		return err
	}
	resp, err := crypto.ComputeResponse(authKey, f.Ciphertext)
	if err != nil {
		return err
	}
	aead, err := crypto.NewAEAD(dataKey)
	if err != nil {
		return err
	}
	nc, err := frame.NewNonceCounter()
	if err != nil {
		return err
	}
	nonce := nc.Next()
	ct, err := aead.Seal(nonce[:], resp, []byte{frame.TypeAuthResponse})
	if err != nil {
		return err
	}
	return frame.WriteFrame(conn, &frame.Frame{
		Type: frame.TypeAuthResponse, Nonce: nonce, Ciphertext: ct,
	})
}

// startTestServer starts a Server on a loopback port. Each authenticated
// connection sends one value on the returned channel.
func startTestServer(t *testing.T) (addr string, authed <-chan struct{}) {
	t.Helper()
	ch := make(chan struct{}, 64)
	srv := &Server{
		PSK:      hsPSK,
		Exporter: func(net.Conn) ([]byte, error) { return hsExporter, nil },
		OnAuthenticated: func(c net.Conn, _ *AuthSession) {
			c.Close()
			ch <- struct{}{}
		},
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go srv.Serve(ln)
	return ln.Addr().String(), ch
}

func TestServerAuthenticatesValidClient(t *testing.T) {
	addr, authed := startTestServer(t)
	if err := hsClientAuth(addr, hsPSK, hsExporter); err != nil {
		t.Fatalf("client: %v", err)
	}
	select {
	case <-authed:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not called for a valid client")
	}
}

func TestServerRejectsWrongPSK(t *testing.T) {
	addr, authed := startTestServer(t)
	bad := bytes.Repeat([]byte{0x99}, 32)
	if err := hsClientAuth(addr, bad, hsExporter); err != nil {
		t.Fatalf("client: %v", err)
	}
	select {
	case <-authed:
		t.Fatal("handler was called for a wrong PSK")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestServerConcurrentClients(t *testing.T) {
	addr, authed := startTestServer(t)
	const n = 20
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() { errs <- hsClientAuth(addr, hsPSK, hsExporter) }()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("client: %v", err)
		}
	}
	for i := 0; i < n; i++ {
		select {
		case <-authed:
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of %d clients authenticated", i, n)
		}
	}
}
