package identity

import (
	"bytes"
	"filippo.io/edwards25519"
)

// ValidPublicKey requires a canonical, nonidentity point in the prime-order subgroup.
// Projecting [8]P back by 8^-1 removes the torsion component; valid P is unchanged.
func ValidPublicKey(raw []byte) bool {
	if len(raw) != 32 {
		return false
	}
	p, e := new(edwards25519.Point).SetBytes(raw)
	if e != nil || !bytes.Equal(p.Bytes(), raw) || p.Equal(edwards25519.NewIdentityPoint()) == 1 {
		return false
	}
	eightBytes := make([]byte, 32)
	eightBytes[0] = 8
	eight, e := new(edwards25519.Scalar).SetCanonicalBytes(eightBytes)
	if e != nil {
		return false
	}
	inverse := new(edwards25519.Scalar).Invert(eight)
	cleared := new(edwards25519.Point).MultByCofactor(p)
	projected := new(edwards25519.Point).ScalarMult(inverse, cleared)
	return projected.Equal(p) == 1
}
