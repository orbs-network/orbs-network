package hash

import (
	"crypto/sha256"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"golang.org/x/crypto/ripemd160"
)

func CalcRipmd160Sha256(data []byte) primitives.Ripmd160Sha256 {
	hash := sha256.Sum256(data)
	return ripemd160.New().Sum(hash[:])
}
