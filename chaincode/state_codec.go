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

func encodeStats(stats Stats) []byte {
	out := make([]byte, 0, 20)
	out = appendUvarint(out, stats.TotalVoteCount)
	return appendUvarint(out, stats.RevealCount)
}

func decodeStats(data []byte) (Stats, error) {
	if len(data) == 0 {
		return Stats{}, nil
	}
	total, n := binary.Uvarint(data)
	if n <= 0 {
		return Stats{}, errors.New("invalid stats total vote count")
	}
	reveals, m := binary.Uvarint(data[n:])
	if m <= 0 {
		return Stats{}, errors.New("invalid stats reveal count")
	}
	return Stats{TotalVoteCount: total, RevealCount: reveals}, nil
}

func loadStats(ctx contractapi.TransactionContextInterface, electionID string) (Stats, error) {
	key, err := statsKey(ctx, electionID)
	if err != nil {
		return Stats{}, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return Stats{}, err
	}
	return decodeStats(data)
}

func saveStats(ctx contractapi.TransactionContextInterface, electionID string, stats Stats) error {
	key, err := statsKey(ctx, electionID)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(key, encodeStats(stats))
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

func encodeUint64(value uint64) []byte {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], value)
	return buf[:n]
}

func decodeUint64(data []byte) (uint64, error) {
	value, n := binary.Uvarint(data)
	if n <= 0 {
		return 0, errors.New("invalid uint64 state")
	}
	return value, nil
}

func loadUint64(ctx contractapi.TransactionContextInterface, key string) (uint64, error) {
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	return decodeUint64(data)
}
