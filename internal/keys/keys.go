package keys

import (
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
	"github.com/number571/go-peer/pkg/crypto/puzzle"
)

func ProofKey(work uint64, pubKey asymmetric.IPubKey) uint64 {
	hash := pubKey.GetHasher().ToBytes()
	return puzzle.NewPoWPuzzle(work).ProofBytes(hash, 32)
}

func VerifyProofKey(work, proof uint64, pubKey asymmetric.IPubKey) bool {
	hash := pubKey.GetHasher().ToBytes()
	return puzzle.NewPoWPuzzle(work).VerifyBytes(hash, proof)
}
