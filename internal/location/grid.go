// Package location converts Maidenhead cells to their center coordinates.
package location

import (
	"errors"
	"fmt"
	"strings"
)

// APRS accepts 4, 6, 8 or 10 characters. Integer arithmetic gives deterministic
// nearest-hundredth-minute rounding, including carries into the next degree.
func APRS(grid string) (latitude, longitude string, err error) {
	g := strings.ToUpper(grid)
	if len(g) < 4 || len(g) > 10 || len(g)%2 != 0 {
		return "", "", errors.New("grid must have 4, 6, 8 or 10 Maidenhead characters")
	}
	parts, lon, lat := int64(1), int64(0), int64(0)
	for pair := 0; pair < len(g)/2; pair++ {
		var factor, base int64
		if pair == 0 {
			factor, base = 18, 'A'
		} else if pair%2 == 1 {
			factor, base = 10, '0'
		} else {
			factor, base = 24, 'A'
		}
		x, y := int64(g[2*pair])-base, int64(g[2*pair+1])-base
		if x < 0 || x >= factor || y < 0 || y >= factor {
			return "", "", errors.New("invalid Maidenhead grid")
		}
		lon = lon*factor + x
		lat = lat*factor + y
		parts *= factor
	}
	return coordinate(90*(2*lat+1-parts), parts, 2, "NS"), coordinate(180*(2*lon+1-parts), parts, 3, "EW"), nil
}
func coordinate(numerator, denominator int64, width int, hemispheres string) string {
	hemisphere := hemispheres[0]
	if numerator < 0 {
		hemisphere = hemispheres[1]
		numerator = -numerator
	}
	value := (numerator*6000 + denominator/2) / denominator
	return fmt.Sprintf("%0*d%02d.%02d%c", width, value/6000, value%6000/100, value%100, hemisphere)
}
