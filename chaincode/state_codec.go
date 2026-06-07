package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func encodeVote(record VoteView) []byte {
	return mustJSONBytes(record)
}

func decodeVote(data []byte) (VoteView, error) {
	var record VoteView
	if err := json.Unmarshal(data, &record); err != nil {
		return VoteView{}, err
	}
	return record, nil
}

func encodeMerkleRoot(record MerkleRootView) []byte {
	return mustJSONBytes(record)
}

func decodeMerkleRoot(data []byte) (MerkleRootView, error) {
	var record MerkleRootView
	if err := json.Unmarshal(data, &record); err != nil {
		return MerkleRootView{}, err
	}
	return record, nil
}

func loadMerkleRoot(ctx contractapi.TransactionContextInterface, electionID string) (MerkleRootView, error) {
	key, err := compositeKey(ctx, rootPrefix, electionID)
	if err != nil {
		return MerkleRootView{}, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return MerkleRootView{}, err
	}
	if len(data) == 0 {
		return MerkleRootView{ElectionID: electionID, Committed: false}, nil
	}
	record, err := decodeMerkleRoot(data)
	if err != nil {
		return MerkleRootView{}, err
	}
	record.Committed = true
	return record, nil
}

// countVotes đếm số record "vote" của một election bằng range query trên composite
// key, thay cho counter dùng chung (vốn gây MVCC_READ_CONFLICT khi vote đồng thời).
func countVotes(ctx contractapi.TransactionContextInterface, electionID string) (uint64, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(votePrefix, []string{electionID})
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	var total uint64
	for iter.HasNext() {
		if _, err := iter.Next(); err != nil {
			return 0, err
		}
		total++
	}
	return total, nil
}

// aggregateReveals duyệt các record usedReveal của election để dựng tally theo từng
// ứng viên và đếm số lá phiếu đã reveal. Vì candidateIds được lưu sẵn trong từng
// record, đây là nguồn sự thật duy nhất — không cần counter tally/RevealCount riêng.
func aggregateReveals(ctx contractapi.TransactionContextInterface, electionID string) (map[string]uint64, uint64, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(usedRevealPrefix, []string{electionID})
	if err != nil {
		return nil, 0, err
	}
	defer iter.Close()

	tally := make(map[string]uint64)
	var revealCount uint64
	for iter.HasNext() {
		kv, err := iter.Next()
		if err != nil {
			return nil, 0, err
		}
		candidateIdsJSON, _, _, err := decodeUsedReveal(kv.Value)
		if err != nil {
			return nil, 0, err
		}
		candidateIDs, err := parseCandidateIds(candidateIdsJSON)
		if err != nil {
			return nil, 0, err
		}
		for _, candidateID := range candidateIDs {
			tally[candidateID]++
		}
		revealCount++
	}
	return tally, revealCount, nil
}

// encodeUsedReveal lưu chuỗi JSON canonical của danh sách ứng viên kèm payload hash và txId.
// Layout: [uvarint(len(candidateIdsJSON))][candidateIdsJSON][payloadHash(32B)][txIdBytes(32B)]
// txID là hex string của Fabric transaction (SHA-256 → 64 hex chars = 32 bytes).
func encodeUsedReveal(candidateIdsJSON string, payloadHash []byte, txID string) []byte {
	txIDBytes, err := hex.DecodeString(txID)
	if err != nil || len(txIDBytes) != 32 {
		txIDBytes = nil
	}
	out := make([]byte, 0, len(candidateIdsJSON)+34+32)
	out = appendUvarint(out, uint64(len(candidateIdsJSON)))
	out = append(out, []byte(candidateIdsJSON)...)
	out = append(out, payloadHash...)
	return append(out, txIDBytes...)
}

// decodeUsedReveal trả (candidateIdsJSON, payloadHash, txID, error).
// Tương thích ngược: record cũ (end+32 bytes) trả txID = ""; record mới (end+64 bytes) trả txID đầy đủ.
func decodeUsedReveal(data []byte) (string, []byte, string, error) {
	candidateLen, n := binary.Uvarint(data)
	if n <= 0 {
		return "", nil, "", errors.New("invalid used reveal candidate length")
	}
	start := n
	end := start + int(candidateLen)
	switch len(data) {
	case end + 32:
		return string(data[start:end]), append([]byte(nil), data[end:]...), "", nil
	case end + 64:
		payloadHash := append([]byte(nil), data[end:end+32]...)
		txID := hex.EncodeToString(data[end+32 : end+64])
		return string(data[start:end]), payloadHash, txID, nil
	default:
		return "", nil, "", errors.New("invalid used reveal state")
	}
}

func appendUvarint(out []byte, value uint64) []byte {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], value)
	return append(out, buf[:n]...)
}
