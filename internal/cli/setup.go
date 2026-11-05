package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"io"
	"path/filepath"
	"strings"
)

func initialize(args []string, input io.Reader, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("data-dir", "./data", "single data directory")
	call := flags.String("callsign", "", "callsign")
	ssid := flags.Uint64("ssid", 15, "SSID")
	grid := flags.String("grid", "", "Maidenhead grid")
	name := flags.String("name", "", "server name")
	host := flags.String("host", "", "public MQTT host")
	port := flags.Uint64("port", 1883, "public MQTT port")
	dev := flags.Bool("dev", false, "use existing localhost:18105 dev")
	origin := flags.String("service-url", "", "advanced service origin")
	root := flags.String("trust-root", "", "explicitly approved public root JSON")
	country := flags.String("country", "CN", "country code")
	km := flags.Uint64("filter-km", 5000, "discovery filter radius")
	owner := flags.String("owner", "", "applicant name")
	email := flags.String("email", "", "applicant email")
	note := flags.String("authorization-note", config.DefaultAuthorizationNote, "callsign and server authorization statement")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(errOut, "invalid init arguments")
		return 2
	}
	if *dev && *origin != "" {
		fmt.Fprintln(errOut, "use either --dev or --service-url")
		return 2
	}
	if *dev {
		*origin = "http://localhost:18105"
	}
	if *root != "" {
		absolute, e := filepath.Abs(*root)
		if e != nil {
			fmt.Fprintln(errOut, "invalid public root path")
			return 1
		}
		*root = absolute
	}
	reader := bufio.NewReader(io.LimitReader(input, 4096))
	if *owner == "" {
		fmt.Fprint(errOut, "负责人姓名：")
		line, e := reader.ReadString('\n')
		if e != nil && line == "" {
			fmt.Fprintln(errOut, "applicant name required (--owner)")
			return 2
		}
		*owner = strings.TrimSpace(line)
	}
	if *email == "" {
		fmt.Fprint(errOut, "负责人邮箱：")
		line, e := reader.ReadString('\n')
		if e != nil && line == "" {
			fmt.Fprintln(errOut, "applicant email required (--email)")
			return 2
		}
		*email = strings.TrimSpace(line)
	}
	applicant := &config.Applicant{Name: *owner, Email: *email, AuthorizationNote: *note}
	if e := applicant.Validate(); e != nil {
		fmt.Fprintln(errOut, e)
		return 2
	}
	simple := config.Simple{Applicant: applicant, Callsign: *call, SSID: *ssid, Grid: *grid, Server: config.Server{Name: *name, Host: *host, Port: *port}, CountryCode: *country, FilterKM: *km, ServiceURL: *origin, TrustedRootFile: *root}
	if e := config.Initialize(*dir, simple); e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	absolute, _ := filepath.Abs(*dir)
	json.NewEncoder(out).Encode(map[string]any{"initialized": true, "data_dir": absolute, "next": "enroll directly; application receipt and machine credentials are managed automatically"})
	return 0
}
