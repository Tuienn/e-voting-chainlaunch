package main

import (
	"crypto/sha256"
	"encoding/binary"
)

// revealPayloadDigest băm trên chuỗi JSON canonical của danh sách ứng viên
// (vd: ["64f...a1","64f...c3"]). Backend phải truyền đúng chuỗi canonical đó.
func revealPayloadDigest(candidateIdsJSON string, h []byte, sPrime []byte) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(candidateIdsJSON)))
	hasher := sha256.New()
	hasher.Write([]byte("reveal-v2"))
	hasher.Write(lenBuf[:])
	hasher.Write([]byte(candidateIdsJSON))
	hasher.Write(h)
	hasher.Write(sPrime)
	return hasher.Sum(nil)
}
