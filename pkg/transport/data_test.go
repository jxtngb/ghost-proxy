package transport

import (
	"bytes"
	"testing"

	"github.com/jxtngb/ghost-proxy/pkg/crypto"
)

func testDataKey() []byte {
	return bytes.Repeat([]byte{0x42}, crypto.KeySize)
}

func TestDataChannelRoundTrip(t *testing.T) {
	key := testDataKey()

	sender, err := NewDataChannel(key)
	if err != nil {
		t.Fatal(err)
	}

	receiver, err := NewDataChannel(key)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("hello ghost proxy")

	var buf bytes.Buffer

	if err := sender.WriteDataFrame(&buf, payload); err != nil {
		t.Fatal(err)
	}

	got, err := receiver.ReadDataFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q, want %q", got, payload)
	}
}
