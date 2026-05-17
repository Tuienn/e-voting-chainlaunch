package main

import (
	"crypto/sha256"
	"encoding/binary"
)

func revealPayloadDigest(candidateID string, h []byte, sPrime []byte) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(candidateID)))
	hasher := sha256.New()
	hasher.Write([]byte("reveal-v1"))
	hasher.Write(lenBuf[:])
	hasher.Write([]byte(candidateID))
	hasher.Write(h)
	hasher.Write(sPrime)
	return hasher.Sum(nil)
}
