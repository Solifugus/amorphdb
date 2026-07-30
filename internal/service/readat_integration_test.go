// Integration test for the READ_AT wire path — a temporal query crossing the
// protocol between a client and the daemon.
//
// This is the gap that made DEVPLAN Step 25 unreachable for real users: the
// interpreter runs client-side against a ProtocolClient standing in as the
// storage tree, so `my.balance[@2026-01-01]` needs READ_AT over the wire. The
// message types existed but neither codec did, the server handler returned 501,
// and the client stub returned an error — so temporal queries worked in-process
// (tests, embedded use) and failed for anyone using the REPL. See Step 30.
package service

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/storage"
)

// roundTrip sends one message and returns the decoded response.
func roundTrip(t *testing.T, conn net.Conn, msgType byte, payload []byte, seq uint32) *protocol.Message {
	t.Helper()

	msg := protocol.CreateMessage(msgType, seq, payload)
	data, err := protocol.EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("send: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp, err := protocol.DecodeMessage(buf[:n])
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestReadAtProtocolIntegration(t *testing.T) {
	config := DefaultConfig()
	config.LocalSocketPath = filepath.Join(t.TempDir(), "socket")
	config.StorageDir = filepath.Join(t.TempDir(), "data")
	config.NetworkPort = 0
	config.HTTPPort = 0
	config.HTTPSPort = 0

	service, err := New(config)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	// Establish the node owner. Without it the connection has no write or read
	// permission and every request comes back 403.
	if _, err := service.InitOwner("readat-integration-test"); err != nil {
		t.Fatalf("init owner: %v", err)
	}

	if err := service.Start(); err != nil {
		t.Fatalf("start service: %v", err)
	}
	defer service.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", service.GetLocalSocketPath())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	// Seed history directly through the tree. The protocol's permission check
	// operates on literal paths, and "my" is an MBL-level concept the wire layer
	// does not resolve — a protocol WRITE to ["my","balance"] comes back 403. This
	// test is about the READ_AT wire path, so the setup uses resolved paths.
	path := []string{"world", "agent", "1", "balance"}
	seedValue := func(data string) {
		t.Helper()
		v := storage.Value{TypeTag: storage.TypeText, Data: []byte(data)}
		if err := service.tree.Write(path, v, 1); err != nil {
			t.Fatalf("seed %q: %v", data, err)
		}
	}

	seedValue("100")
	time.Sleep(20 * time.Millisecond)
	between := time.Now().UTC().UnixMicro()
	time.Sleep(20 * time.Millisecond)
	seedValue("250")

	seq := uint32(1)
	next := func() uint32 { seq++; return seq }

	readAt := func(ts int64) *protocol.Message {
		t.Helper()
		payload, err := protocol.EncodeReadAtMessage(&protocol.ReadAtMessage{Path: path, Timestamp: ts})
		if err != nil {
			t.Fatalf("encode read_at: %v", err)
		}
		return roundTrip(t, conn, protocol.READ_AT, payload, next())
	}

	t.Run("returns the historical value", func(t *testing.T) {
		resp := readAt(between)
		if resp.Type != protocol.READ_AT_RESPONSE {
			t.Fatalf("response type 0x%02x, want READ_AT_RESPONSE", resp.Type)
		}
		decoded, err := protocol.DecodeReadAtResponseMessage(resp.Payload)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got := string(decoded.Value.Data); got != "100" {
			t.Errorf("value as of the earlier instant = %q, want %q", got, "100")
		}
	})

	t.Run("returns the current value for a later instant", func(t *testing.T) {
		resp := readAt(time.Now().UTC().Add(time.Hour).UnixMicro())
		if resp.Type != protocol.READ_AT_RESPONSE {
			t.Fatalf("response type 0x%02x, want READ_AT_RESPONSE", resp.Type)
		}
		decoded, err := protocol.DecodeReadAtResponseMessage(resp.Payload)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got := string(decoded.Value.Data); got != "250" {
			t.Errorf("value as of a future instant = %q, want %q", got, "250")
		}
	})

	t.Run("errors before the first instance", func(t *testing.T) {
		// 1970-ish: earlier than any instance in this store.
		resp := readAt(1)
		if resp.Type != protocol.ERROR {
			t.Fatalf("response type 0x%02x, want ERROR", resp.Type)
		}
		errMsg, err := protocol.DecodeErrorMessage(resp.Payload)
		if err != nil {
			t.Fatalf("decode error message: %v", err)
		}
		if errMsg.Code != 404 {
			t.Errorf("error code = %d, want 404", errMsg.Code)
		}
	})

	t.Run("errors for an unknown path", func(t *testing.T) {
		payload, err := protocol.EncodeReadAtMessage(&protocol.ReadAtMessage{
			Path: []string{"world", "agent", "1", "definitely-missing"}, Timestamp: time.Now().UnixMicro(),
		})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		resp := roundTrip(t, conn, protocol.READ_AT, payload, next())
		if resp.Type != protocol.ERROR {
			t.Errorf("response type 0x%02x, want ERROR for a missing path", resp.Type)
		}
	})
}
