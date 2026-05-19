package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func sha256Bytes(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// hashSortedPair khớp với merkletreejs option sortPairs:true ở off-chain
// (libs/fabric/src/lib/merketree/index.ts): khi băm cặp anh em, hai bên
// được sort theo byte-order trước khi nối → nhờ vậy proof không cần kèm
// position. Phải giữ nguyên thuật toán này, đổi sẽ làm sai gốc Merkle.
func hashSortedPair(a []byte, b []byte) []byte {
	first, second := a, b
	if bytes.Compare(a, b) > 0 {
		first, second = b, a
	}
	return sha256Bytes(append(append([]byte(nil), first...), second...))
}

// parseProof nhận JSON array các hex hash 32-byte (định dạng off-chain trả về
// từ MerkleTree.getHexProof). Mỗi phần tử là sibling hash, không có position
// vì verify dùng sorted-pair.
func parseProof(value string) ([][]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var hexHashes []string
	if err := json.Unmarshal([]byte(value), &hexHashes); err != nil {
		return nil, fmt.Errorf("proof must be JSON array of hex strings: %w", err)
	}
	out := make([][]byte, 0, len(hexHashes))
	for i, raw := range hexHashes {
		clean := strings.TrimPrefix(strings.TrimSpace(raw), "0x")
		decoded, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("proof[%d]: invalid hex: %w", i, err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("proof[%d]: must be 32 bytes, got %d", i, len(decoded))
		}
		out = append(out, decoded)
	}
	return out, nil
}

func applyProof(leaf []byte, proof [][]byte) []byte {
	current := append([]byte(nil), leaf...)
	for _, sibling := range proof {
		current = hashSortedPair(current, sibling)
	}
	return current
}
