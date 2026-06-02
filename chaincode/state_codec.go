package main

import (
	"encoding/binary"
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
		candidateIdsJSON, _, err := decodeUsedReveal(kv.Value)
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

// encodeUsedReveal lưu chuỗi JSON canonical của danh sách ứng viên kèm payload hash.
// Cơ chế uvarint-len đã variable-length nên dùng được cho chuỗi JSON độ dài bất kỳ.
func encodeUsedReveal(candidateIdsJSON string, payloadHash []byte) []byte {
	out := make([]byte, 0, len(candidateIdsJSON)+34)
	out = appendUvarint(out, uint64(len(candidateIdsJSON)))
	out = append(out, []byte(candidateIdsJSON)...)
	return append(out, payloadHash...)
}

func decodeUsedReveal(data []byte) (string, []byte, error) {
	candidateLen, n := binary.Uvarint(data)
	if n <= 0 {
		return "", nil, errors.New("invalid used reveal candidate length")
	}
	start := n
	end := start + int(candidateLen)
	if len(data) != end+32 {
		return "", nil, errors.New("invalid used reveal state")
	}
	return string(data[start:end]), append([]byte(nil), data[end:]...), nil
}

func appendUvarint(out []byte, value uint64) []byte {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], value)
	return append(out, buf[:n]...)
}
