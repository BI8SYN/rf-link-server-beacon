package aprs

import (
	"bufio"
	"context"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"github.com/BI8SYN/rf-link-server-beacon/internal/testutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifiedLoginRequiredAndExactlyOneCRLF(t *testing.T) {
	for _, verified := range []bool{false, true} {
		t.Run(strconv.FormatBool(verified), func(t *testing.T) {
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			received := make(chan string, 1)
			go func() {
				conn, e := listener.Accept()
				if e != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				r := bufio.NewReader(conn)
				login, _ := r.ReadString('\n')
				if !strings.HasPrefix(login, "user BI8SYN-15 pass 12345 vers ") {
					received <- "bad login"
					return
				}
				status := "unverified"
				if verified {
					status = "verified"
				}
				conn.Write([]byte("# logresp BI8SYN-15 " + status + ", server TEST\r\n"))
				line, _ := r.ReadString('\n')
				received <- line
			}()
			p := filepath.Join(t.TempDir(), "passcode")
			os.WriteFile(p, []byte("12345\n"), 0600)
			addr := listener.Addr().(*net.TCPAddr)
			sender := Sender{Host: "127.0.0.1", Port: uint64(addr.Port), PasscodeFile: p}
			now := time.Now()
			if now.Unix()%600 > 590 {
				now = now.Add(20 * time.Second)
			}
			packet := testutil.Announcement(t, now)
			e = sender.Send(context.Background(), packet)
			line := <-received
			if verified {
				if e != nil || line != packet.Line()+"\r\n" {
					t.Fatalf("send %v %q", e, line)
				}
			} else if e == nil || line != "" {
				t.Fatal("unverified session sent payload")
			}
		})
	}
}
func TestRejectZeroOrExpiredAnnouncementBeforeDial(t *testing.T) {
	s := Sender{Host: "127.0.0.1", Port: 1}
	if e := s.Send(context.Background(), signer.Announcement{}); e == nil {
		t.Fatal("unverified packet accepted")
	}
	old := time.Now().Add(-10 * time.Minute)
	if old.Unix()%600 > 590 {
		old = old.Add(20 * time.Second)
	}
	if e := s.Send(context.Background(), testutil.Announcement(t, old)); e == nil {
		t.Fatal("expired packet accepted")
	}
}
func TestCancellationClosesPendingLogin(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, e := l.Accept()
		if e == nil {
			accepted <- c
		}
	}()
	p := filepath.Join(t.TempDir(), "passcode")
	os.WriteFile(p, []byte("12345"), 0600)
	s := Sender{Host: "127.0.0.1", Port: uint64(l.Addr().(*net.TCPAddr).Port), PasscodeFile: p}
	now := time.Now()
	if now.Unix()%600 > 590 {
		now = now.Add(20 * time.Second)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Send(ctx, testutil.Announcement(t, now)) }()
	conn := <-accepted
	defer conn.Close()
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not close socket")
	}
}

func TestSenderCalculatesPasscodeWithoutSecretFile(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	done := make(chan string, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(2 * time.Second))
		reader := bufio.NewReader(c)
		login, _ := reader.ReadString('\n')
		c.Write([]byte("# logresp AB1CD-15 verified, server TEST\r\n"))
		reader.ReadString('\n')
		done <- login
	}()
	now := time.Now()
	if now.Unix()%600 >= 590 {
		now = now.Add(20 * time.Second)
	}
	s := Sender{Host: "127.0.0.1", Port: uint64(l.Addr().(*net.TCPAddr).Port)}
	if e = s.Send(context.Background(), testutil.AnnouncementForCallsign(t, now, "AB1CD")); e != nil {
		t.Fatal(e)
	}
	if login := <-done; login != "user AB1CD-15 pass 18403 vers RFLinkBeacon 0.1\r\n" {
		t.Fatal("automatic APRS login does not match independent checksum vector")
	}
}
