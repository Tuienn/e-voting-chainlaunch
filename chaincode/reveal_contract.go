package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// parseCandidateIds giải mã chuỗi JSON canonical (vd: ["a","b"]) thành []string và
// kiểm tra cơ bản: mảng không rỗng và từng phần tử không rỗng. Việc enforce
// maxSelectableCandidates / candidate ∈ election do backend đảm nhiệm (chaincode tối giản).
func parseCandidateIds(candidateIdsJSON string) ([]string, error) {
	if err := requireNonEmpty(candidateIdsJSON, "candidateIds"); err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(candidateIdsJSON), &ids); err != nil {
		return nil, fmt.Errorf("candidateIds must be a JSON array of strings: %w", err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("candidateIds must not be empty")
	}
	for _, id := range ids {
		if err := requireNonEmpty(id, "candidateId"); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func (c *VoteLedgerContract) RevealVoteCompact(ctx contractapi.TransactionContextInterface, electionID, candidateIdsJSON, revealKey, revealPayloadHash string) (string, error) {
	if err := requireNonEmpty(electionID, "electionId"); err != nil {
		return "", err
	}
	candidateIDs, err := parseCandidateIds(candidateIdsJSON)
	if err != nil {
		return "", err
	}
	keyHash, err := parseHash32(revealKey, "revealKey")
	if err != nil {
		return "", err
	}
	payloadHash, err := parseHash32(revealPayloadHash, "revealPayloadHash")
	if err != nil {
		return "", err
	}
	root, err := loadMerkleRoot(ctx, electionID)
	if err != nil {
		return "", err
	}
	if !root.Committed {
		return "", fmt.Errorf("election %s has no Merkle root; reveal is not allowed", electionID)
	}

	usedKey, err := compositeKey(ctx, usedRevealPrefix, electionID, hashToBase64URL(keyHash))
	if err != nil {
		return "", err
	}
	existing, err := ctx.GetStub().GetState(usedKey)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", fmt.Errorf("revealKey %s has already been used", hashToBase64URL(keyHash))
	}

	// Chỉ ghi MỘT key duy nhất cho mỗi ballot: usedReveal (key = hash revealKey).
	// Bản thân record đã chứa candidateIdsJSON, nên tally từng ứng viên và RevealCount
	// đều được tổng hợp on-demand từ các record usedReveal (xem aggregateReveals).
	// Bỏ counter dùng chung (tally per candidate, stats.RevealCount) để các reveal
	// đồng thời không đụng key chung -> tránh MVCC_READ_CONFLICT.
	txID := ctx.GetStub().GetTxID()
	if err := ctx.GetStub().PutState(usedKey, encodeUsedReveal(candidateIdsJSON, payloadHash, txID)); err != nil {
		return "", err
	}

	return mustJSON(UsedRevealView{
		ElectionID:         electionID,
		CandidateIDs:       candidateIDs,
		RevealKey:          hashToBase64URL(keyHash),
		RevealKeyHex:       hex.EncodeToString(keyHash),
		RevealPayloadHash:  hashToBase64URL(payloadHash),
		RevealPayloadHashH: hex.EncodeToString(payloadHash),
		TransactionID:      txID,
	}), nil
}

func (c *VoteLedgerContract) GetTally(ctx contractapi.TransactionContextInterface, electionID string) (string, error) {
	if err := requireNonEmpty(electionID, "electionId"); err != nil {
		return "", err
	}
	tally, _, err := aggregateReveals(ctx, electionID)
	if err != nil {
		return "", err
	}
	return mustJSON(TallyView{ElectionID: electionID, Tally: tally}), nil
}

func (c *VoteLedgerContract) GetUsedReveal(ctx contractapi.TransactionContextInterface, electionID, revealKey string) (string, error) {
	keyHash, err := parseHash32(revealKey, "revealKey")
	if err != nil {
		return "", err
	}
	key, err := compositeKey(ctx, usedRevealPrefix, electionID, hashToBase64URL(keyHash))
	if err != nil {
		return "", err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("revealKey %s has not been used", hashToBase64URL(keyHash))
	}
	candidateIdsJSON, payloadHash, txID, err := decodeUsedReveal(data)
	if err != nil {
		return "", err
	}
	candidateIDs, err := parseCandidateIds(candidateIdsJSON)
	if err != nil {
		return "", err
	}
	return mustJSON(UsedRevealView{
		ElectionID:         electionID,
		CandidateIDs:       candidateIDs,
		RevealKey:          hashToBase64URL(keyHash),
		RevealKeyHex:       hex.EncodeToString(keyHash),
		RevealPayloadHash:  hashToBase64URL(payloadHash),
		RevealPayloadHashH: hex.EncodeToString(payloadHash),
		TransactionID:      txID,
	}), nil
}

func (c *VoteLedgerContract) ComputeRevealPayloadHash(ctx contractapi.TransactionContextInterface, candidateIdsJSON, h, sPrime string) (string, error) {
	candidateIDs, err := parseCandidateIds(candidateIdsJSON)
	if err != nil {
		return "", err
	}
	hBytes, err := parseHash32(h, "h")
	if err != nil {
		return "", err
	}
	sPrimeBytes, err := parseHash32(sPrime, "sPrime")
	if err != nil {
		return "", err
	}
	hash := revealPayloadDigest(candidateIdsJSON, hBytes, sPrimeBytes)
	return mustJSON(PayloadHashView{
		CandidateIDs:         candidateIDs,
		RevealPayloadHash:    hashToBase64URL(hash),
		RevealPayloadHashHex: hex.EncodeToString(hash),
		HashDefinition:       "sha256('reveal-v2' || uint32be(len(candidateIdsJson)) || candidateIdsJson || h32 || sPrime32)",
	}), nil
}
