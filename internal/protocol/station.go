package protocol

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxInteger uint64 = 1<<53 - 1

var callsignPattern = regexp.MustCompile(`^[A-Z0-9]{1,12}$`)
var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func B64(b []byte) string  { return base64.RawURLEncoding.EncodeToString(b) }
func Hash(b []byte) string { h := sha256.Sum256(b); return B64(h[:]) }
func Decode64(s string, n int) ([]byte, error) {
	b, e := base64.RawURLEncoding.Strict().DecodeString(s)
	if e != nil || len(b) == 0 || (n > 0 && len(b) != n) || B64(b) != s {
		return nil, errors.New("invalid canonical base64url")
	}
	return b, nil
}
func ValidCallsign(s string) bool { return callsignPattern.MatchString(s) }
func ValidHost(s string) bool {
	if len(s) == 0 || len(s) > 253 || s != strings.ToLower(s) {
		return false
	}
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	if a, e := netip.ParseAddr(s); e == nil {
		return a.String() == s && !strings.Contains(s, "%")
	}
	if strings.Contains(s, ":") {
		return false
	}
	onlyNumbers := true
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			onlyNumbers = false
		}
	}
	if onlyNumbers {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if !labelPattern.MatchString(label) {
			return false
		}
	}
	return true
}

type Station struct {
	SSID        uint64 `json:"ssid"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
	SymbolTable string `json:"symbolTable"`
	SymbolCode  string `json:"symbolCode"`
	CountryCode string `json:"countryCode"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        uint64 `json:"port"`
	FilterKM    uint64 `json:"filterKm"`
}

func (s Station) Validate() error {
	if s.SSID > 15 || s.Port == 0 || s.Port > 65535 || s.FilterKM > 100000 || s.SymbolTable != "/" || s.SymbolCode != "I" || !coordinate(s.Latitude, 2, 90, "NS") || !coordinate(s.Longitude, 3, 180, "EW") || len(s.CountryCode) != 2 || strings.Trim(s.CountryCode, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" || !ValidHost(s.Host) {
		return errors.New("invalid station fields")
	}
	if !ValidText(s.Name, 64) {
		return errors.New("invalid station name")
	}
	return nil
}
func ValidText(s string, max int) bool {
	if s == "" || len(s) > max || !utf8.ValidString(s) || strings.TrimSpace(s) != s {
		return false
	}
	for _, c := range s {
		if c == ',' || unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			return false
		}
	}
	return true
}
func coordinate(s string, digits, max int, hemi string) bool {
	if len(s) != digits+6 || s[digits+2] != '.' || !strings.ContainsRune(hemi, rune(s[len(s)-1])) {
		return false
	}
	for i, c := range s[:len(s)-1] {
		if i != digits+2 && (c < '0' || c > '9') {
			return false
		}
	}
	d, _ := strconv.Atoi(s[:digits])
	m, _ := strconv.Atoi(s[digits:digits+2] + s[digits+3:digits+5])
	return d <= max && m < 6000 && (d < max || m == 0)
}

type UserCert struct {
	IssuerSN  uint64
	Callsign  string
	UID       uint64
	PublicKey []byte
	IAT       uint64
	EXP       uint64
	Signature []byte
}

func (c UserCert) TBS() []byte {
	return Encode([]any{"FMO", 4, "userCert", c.IssuerSN, c.Callsign, c.UID, c.PublicKey, c.IAT, c.EXP})
}
func (c UserCert) Blob() []byte {
	return Encode([]any{"FMO", 4, "userCert", c.IssuerSN, c.Callsign, c.UID, c.PublicKey, c.IAT, c.EXP, c.Signature})
}
func (c UserCert) Fingerprint() string { return Hash(c.TBS()) }
func (c UserCert) BlobHash() string    { return Hash(c.Blob()) }
func ParseBlob(s string) (UserCert, error) {
	var c UserCert
	b, e := Decode64(s, 0)
	if e != nil || len(b) > 1024 {
		return c, errors.New("invalid CERT")
	}
	v, e := decode(b)
	if e != nil {
		return c, e
	}
	a, ok := v.([]any)
	if !ok || len(a) != 10 {
		return c, errors.New("CERT requires ten items")
	}
	if a[0] != "FMO" || a[1] != uint64(4) || a[2] != "userCert" {
		return c, errors.New("invalid CERT domain")
	}
	sn, o1 := a[3].(uint64)
	call, o2 := a[4].(string)
	uid, o3 := a[5].(uint64)
	pub, o4 := a[6].([]byte)
	iat, o5 := a[7].(uint64)
	exp, o6 := a[8].(uint64)
	sig, o7 := a[9].([]byte)
	if !o1 || !o2 || !o3 || !o4 || !o5 || !o6 || !o7 || !ValidCallsign(call) || uid > 0xffffffff || len(pub) != 32 || len(sig) != 64 || iat >= exp {
		return c, errors.New("invalid CERT fields")
	}
	return UserCert{sn, call, uid, pub, iat, exp, sig}, nil
}
func StationTBS(c UserCert, s Station, online, peak, salt uint64) []byte {
	hash := sha256.Sum256(c.Blob())
	return Encode([]any{"FMO", 4, "STATION", c.Callsign, s.SSID, s.Latitude, s.Longitude, hash[:], s.CountryCode, s.Name, s.Host, s.Port, s.FilterKM, online, peak, salt})
}
func Line(c UserCert, s Station, online, peak uint64, signature string) (string, error) {
	if e := s.Validate(); e != nil {
		return "", e
	}
	if online > peak || peak > MaxInteger || !ValidCallsign(c.Callsign) {
		return "", errors.New("invalid station identity or counts")
	}
	if _, e := Decode64(signature, 64); e != nil {
		return "", e
	}
	source := c.Callsign
	if s.SSID != 0 {
		source += fmt.Sprintf("-%d", s.SSID)
	}
	line := fmt.Sprintf("%s>APFMO4,TCPIP*:=%s/%sIFMO-V4,STATION,CERT:%s,%s,%s,%s,P%d,F%dKM,U%d/%d,SIG:%s", source, s.Latitude, s.Longitude, B64(c.Blob()), s.CountryCode, s.Name, s.Host, s.Port, s.FilterKM, online, peak, signature)
	if len(line) > 480 {
		return "", errors.New("station exceeds 480 UTF-8 bytes")
	}
	return line, nil
}
