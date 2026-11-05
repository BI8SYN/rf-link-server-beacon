// Package aprs publishes one verified STATION after APRS-IS login confirmation.
package aprs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type Sender struct {
	Host         string
	Port         uint64
	PasscodeFile string
}

func (s Sender) Send(ctx context.Context, packet signer.Announcement) error {
	if packet.Line() == "" || !time.Now().Before(packet.Deadline()) || strings.ContainsAny(packet.Line(), "\r\n") {
		return errors.New("APRS packet is unverified or expired")
	}
	code, e := Passcode(packet.Source())
	if e != nil {
		return e
	}
	pass := strconv.FormatUint(uint64(code), 10)
	if s.PasscodeFile != "" {
		pass, e = config.Secret(s.PasscodeFile)
		if e != nil {
			return e
		}
	}
	if n, e := strconv.ParseUint(pass, 10, 16); e != nil || n > 32767 {
		return errors.New("APRS send passcode must be a valid nonnegative number")
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, e := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.Host, strconv.FormatUint(s.Port, 10)))
	if e != nil {
		return errors.New("APRS connection failed")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline := time.Now().Add(10 * time.Second)
	if packet.Deadline().Before(deadline) {
		deadline = packet.Deadline()
	}
	conn.SetDeadline(deadline)
	login := fmt.Sprintf("user %s pass %s vers RFLinkBeacon 0.1\r\n", packet.Source(), pass)
	if e = write(conn, login); e != nil {
		return errors.New("APRS login write failed")
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024), 1024)
	for count := 0; count < 32 && scanner.Scan(); count++ {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 && fields[0] == "#" && fields[1] == "logresp" {
			if fields[2] != packet.Source() || strings.TrimSuffix(fields[3], ",") != "verified" {
				return errors.New("APRS login was not verified for this source")
			}
			if ctx.Err() != nil || !time.Now().Before(packet.Deadline()) {
				return errors.New("APRS packet expired before send")
			}
			if e = write(conn, packet.Line()+"\r\n"); e != nil {
				return errors.New("APRS send outcome unknown; packet will not be queued")
			}
			return nil
		}
	}
	return errors.New("APRS login confirmation unavailable")
}
func write(w io.Writer, s string) error {
	for len(s) > 0 {
		n, e := io.WriteString(w, s)
		if e != nil {
			return e
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		s = s[n:]
	}
	return nil
}
