package config

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"github.com/BI8SYN/rf-link-server-beacon/internal/location"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DefaultOrigin is the existing RF Link Trust business origin. The broadcast API
// hosts the enrollment, identity and restricted STATION signing APIs.
const DefaultOrigin = "https://trust.bi8syn.com"

//go:embed assets/rf-link-root-production-v1.json
var bundledRoot []byte

type Server struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port uint64 `json:"port,omitempty"`
}
type Simple struct {
	Applicant       *Applicant `json:"applicant,omitempty"`
	Callsign        string     `json:"callsign"`
	SSID            uint64     `json:"ssid,omitempty"`
	Grid            string     `json:"grid"`
	Server          Server     `json:"server"`
	CountryCode     string     `json:"country_code,omitempty"`
	FilterKM        uint64     `json:"filter_km,omitempty"`
	ServiceURL      string     `json:"service_url,omitempty"`
	TrustedRootFile string     `json:"trusted_root_file,omitempty"`
	CRLOrigins      []string   `json:"crl_origins,omitempty"`
}

func simpleDefaults() Simple {
	return Simple{SSID: 15, Server: Server{Port: 1883}, CountryCode: "CN", FilterKM: 5000}
}
func loadSimple(raw []byte, path string) (Config, error) {
	simple := simpleDefaults()
	if e := jsonx.Decode(raw, &simple); e != nil {
		return Config{}, errors.New("invalid simplified configuration JSON or fields")
	}
	dir, e := filepath.Abs(filepath.Dir(path))
	if e != nil {
		return Config{}, e
	}
	return simple.expand(dir)
}
func (s Simple) expand(dir string) (Config, error) {
	var c Config
	lat, lon, e := location.APRS(s.Grid)
	if e != nil {
		return c, e
	}
	origin := s.ServiceURL
	if origin == "" {
		origin = DefaultOrigin
	}
	c.Applicant = s.Applicant
	if c.Applicant != nil {
		if e = c.Applicant.Validate(); e != nil {
			return c, e
		}
	}
	c.Schema = 1
	c.CoordinatesConfirmed = true
	c.Callsign = strings.ToUpper(s.Callsign)
	c.Label = "服务器公告身份"
	c.PurposeNote = "用于本服务器STATION发现公告"
	c.Signer.BaseURL = origin
	c.Signer.CredentialFile = filepath.Join(dir, "credential")
	u, e := url.Parse(origin)
	if e != nil {
		return c, errors.New("invalid service URL")
	}
	c.Signer.AllowLoopbackHTTP = u.Scheme == "http"
	c.StateDir = filepath.Join(dir, "state")
	country := s.CountryCode
	if country == "" {
		country = "CN"
	}
	c.Station = protocol.Station{SSID: s.SSID, Latitude: lat, Longitude: lon, SymbolTable: "/", SymbolCode: "I", CountryCode: country, Name: s.Server.Name, Host: strings.ToLower(s.Server.Host), Port: s.Server.Port, FilterKM: s.FilterKM}
	c.APRS.Host = "rotate.aprs2.net"
	c.APRS.Port = 14580
	c.APRS.Interval = 600
	raw := bundledRoot
	if s.TrustedRootFile != "" {
		path := s.TrustedRootFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		raw, e = readPublicRoot(path)
		if e != nil {
			return c, e
		}
	}
	var root identity.Root
	if e = jsonx.Decode(raw, &root); e != nil {
		return c, errors.New("trusted root file must contain one public FMO root certificate")
	}
	pub, e := protocol.Decode64(root.Subject.PublicKey, 32)
	if e != nil || !identity.ValidPublicKey(pub) {
		return c, errors.New("invalid trusted root key")
	}
	sig, e := protocol.Decode64(root.Signature, 64)
	if e != nil || root.Type != "rootCA" || root.Algorithm != "Ed25519" || !root.Extensions.IsCA || root.Extensions.PathLen != 1 || !ed25519.Verify(pub, root.TBS(), sig) {
		return c, errors.New("invalid trusted root self signature")
	}
	c.Trust.Roots = []string{protocol.Hash(root.TBS())}
	crlOrigin, e := Origin(root.Extensions.CRL)
	if e != nil {
		return c, e
	}
	c.Trust.CRLOrigins = []string{crlOrigin}
	c.Trust.CRLOrigins = append(c.Trust.CRLOrigins, s.CRLOrigins...)
	return c, c.Validate()
}
func Origin(source string) (string, error) {
	u, e := url.Parse(source)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid CRL origin")
	}
	return u.Scheme + "://" + u.Host, nil
}
func (c Config) AllowCRL(source string) bool {
	if URL(source, c.Signer.AllowLoopbackHTTP) != nil {
		return false
	}
	for _, exact := range c.Trust.CRLURLs {
		if source == exact {
			return true
		}
	}
	origin, e := Origin(source)
	if e != nil {
		return false
	}
	for _, allowed := range c.Trust.CRLOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}
func readPublicRoot(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.New("cannot open explicitly trusted public root file")
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 65537))
	if e != nil || len(raw) > 65536 {
		return nil, errors.New("public root file exceeds limit")
	}
	return raw, nil
}

// Initialize never replaces existing configuration. Custom roots are copied into
// the same directory so the deployment needs just one bind mount.
func Initialize(dir string, s Simple) error {
	var e error
	dir, e = filepath.Abs(dir)
	if e != nil {
		return e
	}
	if _, e = os.Lstat(filepath.Join(dir, "config.json")); e == nil {
		return errors.New("configuration already exists; edit it explicitly instead of reinitializing")
	}
	if s.CountryCode == "" {
		s.CountryCode = "CN"
	}
	if _, e = s.expand(dir); e != nil {
		return e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() {
		return errors.New("data directory must be a real directory")
	}
	if e = os.Chmod(dir, 0700); e != nil {
		return e
	}
	if s.TrustedRootFile != "" {
		path := s.TrustedRootFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		raw, err := readPublicRoot(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, "trusted-root.json")
		if filepath.Clean(path) != dest {
			if err = writeNew(dest, raw); err != nil {
				return err
			}
		}
		s.TrustedRootFile = "trusted-root.json"
	}
	fields := map[string]any{"callsign": strings.ToUpper(s.Callsign), "ssid": s.SSID, "grid": strings.ToUpper(s.Grid), "server": s.Server}
	if s.Applicant != nil {
		fields["applicant"] = s.Applicant
	}
	if s.CountryCode != "CN" {
		fields["country_code"] = s.CountryCode
	}
	if s.FilterKM != 5000 {
		fields["filter_km"] = s.FilterKM
	}
	if s.ServiceURL != "" {
		fields["service_url"] = s.ServiceURL
	}
	if s.TrustedRootFile != "" {
		fields["trusted_root_file"] = s.TrustedRootFile
	}
	if len(s.CRLOrigins) > 0 {
		fields["crl_origins"] = s.CRLOrigins
	}
	raw, e := json.MarshalIndent(fields, "", "  ")
	if e != nil {
		return e
	}
	return writeNew(filepath.Join(dir, "config.json"), append(raw, '\n'))
}
func writeNew(path string, raw []byte) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(raw); e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(path)
	}
	return e
}

// ApplyManagedSettings updates only station fields in an existing simple
// configuration. The originally submitted applicant and callsign are durable.
func ApplyManagedSettings(path string, requested Simple) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	saved := simpleDefaults()
	if err = jsonx.Decode(raw, &saved); err != nil {
		return errors.New("managed deployment requires its original simple configuration")
	}
	if strings.ToUpper(requested.Callsign) != saved.Callsign {
		return errors.New("BEACON_CALLSIGN differs from the existing identity")
	}
	saved.Server = requested.Server
	saved.SSID = requested.SSID
	saved.Grid = strings.ToUpper(requested.Grid)
	saved.CountryCode = requested.CountryCode
	if _, err = saved.expand(filepath.Dir(path)); err != nil {
		return err
	}
	// Always include SSID zero because the simplified loader defaults to 15.
	fields := map[string]any{
		"callsign": saved.Callsign, "ssid": saved.SSID, "grid": saved.Grid, "server": saved.Server,
		"applicant": saved.Applicant, "country_code": saved.CountryCode, "filter_km": saved.FilterKM,
	}
	if saved.ServiceURL != "" {
		fields["service_url"] = saved.ServiceURL
	}
	if saved.TrustedRootFile != "" {
		fields["trusted_root_file"] = saved.TrustedRootFile
	}
	if len(saved.CRLOrigins) > 0 {
		fields["crl_origins"] = saved.CRLOrigins
	}
	next, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if _, err = f.Write(append(next, '\n')); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
