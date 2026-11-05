package aprs

import (
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"strconv"
	"strings"
)

// Passcode is the APRS-IS callsign checksum. It is not the Trust machine credential.
func Passcode(source string) (uint16, error) {
	parts := strings.Split(strings.ToUpper(source), "-")
	if len(parts) > 2 || !protocol.ValidCallsign(parts[0]) {
		return 0, errors.New("invalid APRS source")
	}
	if len(parts) == 2 {
		ssid, e := strconv.ParseUint(parts[1], 10, 8)
		if e != nil || ssid > 15 || strconv.FormatUint(ssid, 10) != parts[1] {
			return 0, errors.New("invalid APRS SSID")
		}
	}
	hash := uint16(0x73e2)
	for i, c := range []byte(parts[0]) {
		if i%2 == 0 {
			hash ^= uint16(c) << 8
		} else {
			hash ^= uint16(c)
		}
	}
	return hash & 0x7fff, nil
}
