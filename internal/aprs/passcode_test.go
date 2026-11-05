package aprs

import "testing"

// Values come from the independently maintained APRSdroid tests.
func TestPasscodeIgnoresSSIDAndCase(t *testing.T) {
	for _, call := range []string{"AB1CD", "ab1cd", "AB1CD-15"} {
		pass, e := Passcode(call)
		if e != nil || pass != 18403 {
			t.Fatalf("passcode mismatch for %s: %d %v", call, pass, e)
		}
	}
	for _, call := range []string{"", "BAD CALL", "BI8SYN\r\n", "BI8SYN-16"} {
		if _, e := Passcode(call); e == nil {
			t.Fatalf("invalid source accepted: %q", call)
		}
	}
}
