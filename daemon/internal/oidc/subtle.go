package oidc

import "crypto/subtle"

// subtleStringEqual reports whether a and b are equal in constant time with
// respect to their contents, avoiding a timing side channel on the nonce
// comparison. Length is not secret here, but using ConstantTimeCompare keeps
// the comparison uniform.
func subtleStringEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
