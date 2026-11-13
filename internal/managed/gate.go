package managed

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const deny = `{"result":"deny"}`

type gate struct {
	broker, sas, bootFile, loadedFile string
	client                            *http.Client
	mu                                sync.Mutex
	identity, readyBoot, probeUser    string
	probeDenied                       bool
	readyUntil                        time.Time
}

func newGate(broker, sas, boot, loaded string) *gate {
	return &gate{broker: broker, sas: sas, bootFile: boot, loadedFile: loaded, client: &http.Client{
		Timeout:       2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func marker(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 129))
	if err != nil || len(raw) > 128 {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func (g *gate) setIdentity(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.identity != id {
		g.identity = id
		g.readyBoot = ""
	}
}

func (g *gate) allowed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.readyBoot != "" && time.Now().Before(g.readyUntil) &&
		marker(g.bootFile) == g.readyBoot && marker(g.loadedFile) == g.identity
}

// ServeHTTP preserves the SAS response, including ACL and role fields. An
// unready stack or unexpected SAS response explicitly denies authentication.
func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := []byte(deny)
	defer func() { w.Write(reply) }()
	boot := marker(g.bootFile)
	if boot == "" || r.Method != "POST" || r.URL.Path != "/auth/"+boot {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	var input struct {
		Username string `json:"username"`
	}
	if err != nil || len(raw) > 65536 || json.Unmarshal(raw, &input) != nil {
		return
	}
	g.mu.Lock()
	probe := g.probeUser != "" && input.Username == g.probeUser
	identity := g.identity
	g.mu.Unlock()
	if identity == "" || marker(g.loadedFile) != identity || (!probe && !g.allowed()) {
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), "POST", g.sas+"/auth", bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	var result struct {
		Result string `json:"result"`
	}
	if err != nil || res.StatusCode != 200 || len(data) > 65536 || json.Unmarshal(data, &result) != nil {
		return
	}
	if probe {
		g.mu.Lock()
		if g.probeUser == input.Username && result.Result == "deny" && boot == marker(g.bootFile) && identity == g.identity {
			g.probeDenied = true
		}
		g.mu.Unlock()
		return
	}
	if (result.Result == "allow" || result.Result == "deny") && g.allowed() {
		reply = data
	}
}

func (g *gate) check(ctx context.Context) error {
	boot := marker(g.bootFile)
	g.mu.Lock()
	identity := g.identity
	g.mu.Unlock()
	if boot == "" || identity == "" || marker(g.loadedFile) != identity {
		return errors.New("waiting for SAS and broker startup")
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	name := "probe-" + hex.EncodeToString(nonce[:])
	g.mu.Lock()
	g.probeUser = name
	g.probeDenied = false
	g.mu.Unlock()
	denied := mqttDenied(ctx, g.broker, name)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.probeUser = ""
	if !denied || !g.probeDenied || marker(g.bootFile) != boot || marker(g.loadedFile) != identity || g.identity != identity {
		g.readyBoot = ""
		return errors.New("MQTT authentication callback not ready")
	}
	g.readyBoot = boot
	g.readyUntil = time.Now().Add(8 * time.Second)
	return nil
}

// An invalid synthetic credential must reach SAS and be rejected by EMQX.
func mqttDenied(ctx context.Context, addr, user string) bool {
	d := net.Dialer{Timeout: time.Second}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	packet := []byte{0, 4, 'M', 'Q', 'T', 'T', 4, 0xc2, 0, 5}
	for _, v := range []string{user, user, "invalid"} {
		packet = append(packet, byte(len(v)>>8), byte(len(v)))
		packet = append(packet, v...)
	}
	if _, err = c.Write(append([]byte{0x10, byte(len(packet))}, packet...)); err != nil {
		return false
	}
	reply := make([]byte, 4)
	_, err = io.ReadFull(c, reply)
	return err == nil && reply[0] == 0x20 && reply[1] == 2 && reply[2] == 0 && (reply[3] == 4 || reply[3] == 5)
}

func (g *gate) relay(ctx context.Context, l net.Listener) error {
	stop := context.AfterFunc(ctx, func() { l.Close() })
	defer stop()
	slots := make(chan struct{}, 1024)
	for {
		c, err := l.Accept()
		if err != nil {
			return fmt.Errorf("MQTT listener: %w", err)
		}
		if !g.allowed() {
			c.Close()
			continue
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots }()
				defer c.Close()
				d := net.Dialer{Timeout: 2 * time.Second}
				upstream, err := d.DialContext(ctx, "tcp", g.broker)
				if err != nil {
					return
				}
				defer upstream.Close()
				if !g.allowed() {
					return
				}
				stop := context.AfterFunc(ctx, func() { c.Close(); upstream.Close() })
				defer stop()
				done := make(chan struct{})
				go func() { io.Copy(upstream, c); upstream.Close(); close(done) }()
				io.Copy(c, upstream)
				c.Close()
				<-done
			}()
		default:
			c.Close()
		}
	}
}
