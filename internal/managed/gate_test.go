package managed

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGateRequiresRealAuthForCurrentBrokerAndSAS(t *testing.T) {
	dir := t.TempDir()
	boot := filepath.Join(dir, "boot")
	loaded := filepath.Join(dir, "loaded")
	for path, value := range map[string]string{boot: "boot1", loaded: "identity1"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"result":"deny"}`)
	}))
	defer sas.Close()
	broker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	g := newGate(broker.Addr().String(), sas.URL, boot, loaded)
	g.setIdentity("identity1")
	auth := httptest.NewServer(g)
	defer auth.Close()
	probe := func(callback bool) {
		go func() {
			c, err := broker.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			header := make([]byte, 2)
			if _, err = io.ReadFull(c, header); err != nil {
				return
			}
			payload := make([]byte, int(header[1]))
			if _, err = io.ReadFull(c, payload); err != nil {
				return
			}
			if callback {
				// The test probe has a fixed MQTT header and client ID; parse username.
				off := 10
				off += 2 + int(payload[off+1])
				user := string(payload[off+2 : off+2+int(payload[off+1])])
				resp, e := http.Post(auth.URL+"/auth/boot1", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":"test"}`, user)))
				if e == nil {
					resp.Body.Close()
				}
			}
			c.Write([]byte{0x20, 0x02, 0x00, 0x05})
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe(false)
	if g.check(ctx) == nil || g.allowed() {
		t.Fatal("MQTT denial alone opened gate")
	}
	probe(true)
	if err := g.check(ctx); err != nil || !g.allowed() {
		t.Fatalf("real SAS denial not verified: %v", err)
	}
	os.WriteFile(boot, []byte("boot2"), 0600)
	if g.allowed() {
		t.Fatal("new broker startup reused old readiness")
	}
	os.WriteFile(boot, []byte("boot1"), 0600)
	os.WriteFile(loaded, []byte("identity2"), 0600)
	if g.allowed() {
		t.Fatal("different SAS configuration reused readiness")
	}
}
