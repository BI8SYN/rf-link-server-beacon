package location

import "testing"

func TestGridCentersAndAPRSCarries(t *testing.T) {
	for _, tc := range []struct{ grid, lat, lon string }{{"FN31pr", "4143.75N", "07242.50W"}, {"AA00", "8930.00S", "17900.00W"}, {"RR99xx", "8958.75N", "17957.50E"}, {"jj00aa00", "0000.13N", "00000.25E"}} {
		t.Run(tc.grid, func(t *testing.T) {
			lat, lon, e := APRS(tc.grid)
			if e != nil || lat != tc.lat || lon != tc.lon {
				t.Fatalf("%s %s %v", lat, lon, e)
			}
		})
	}
	for _, s := range []string{"", "A", "SS00", "FN3a", "FN31zz", "FN31pr1", "FN31prAA", "FN31\n"} {
		if _, _, e := APRS(s); e == nil {
			t.Fatalf("invalid grid accepted: %q", s)
		}
	}
}
