package main

import (
	"encoding/hex"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func (c *VoteLedgerContract) RevealVoteCompact(ctx contractapi.TransactionContextInterface, electionID, candidateID, revealKey, revealPayloadHash string) (string, error) {
	if err := requireNonEmpty(electionID, "electionId"); err != nil {
		return "", err
	}
	if err := requireNonEmpty(candidateID, "candidateId"); err != nil {
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

	if err := ctx.GetStub().PutState(usedKey, encodeUsedReveal(candidateID, payloadHash)); err != nil {
		return "", err
	}

	tallyKey, err := compositeKey(ctx, tallyPrefix, electionID, candidateID)
	if err != nil {
		return "", err
	}
	count, err := loadUint64(ctx, tallyKey)
	if err != nil {
		return "", err
	}
	if err := ctx.GetStub().PutState(tallyKey, encodeUint64(count+1)); err != nil {
		return "", err
	}

	stats, err := loadStats(ctx, electionID)
	if err != nil {
		return "", err
	}
	stats.RevealCount++
	if err := saveStats(ctx, electionID, stats); err != nil {
		return "", err
	}

	return mustJSON(UsedRevealView{
		ElectionID:         electionID,
		CandidateID:        candidateID,
		RevealKey:          hashToBase64URL(keyHash),
		RevealKeyHex:       hex.EncodeToString(keyHash),
		RevealPayloadHash:  hashToBase64URL(payloadHash),
		RevealPayloadHashH: hex.EncodeToString(payloadHash),
	}), nil
}

func (c *VoteLedgerContract) GetTally(ctx contractapi.TransactionContextInterface, electionID string) (string, error) {
	if err := requireNonEmpty(electionID, "electionId"); err != nil {
		return "", err
	}
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(tallyPrefix, []string{electionID})
	if err != nil {
		return "", err
	}
	defer iter.Close()

	tally := make(map[string]uint64)
	for iter.HasNext() {
		kv, err := iter.Next()
		if err != nil {
			return "", err
		}
		_, parts, err := ctx.GetStub().SplitCompositeKey(kv.Key)
		if err != nil {
			return "", err
		}
		if len(parts) != 2 {
			continue
		}
		count, err := decodeUint64(kv.Value)
		if err != nil {
			return "", err
		}
		tally[parts[1]] = count
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
	candidateID, payloadHash, err := decodeUsedReveal(data)
	if err != nil {
		return "", err
	}
	return mustJSON(UsedRevealView{
		ElectionID:         electionID,
		CandidateID:        candidateID,
		RevealKey:          hashToBase64URL(keyHash),
		RevealKeyHex:       hex.EncodeToString(keyHash),
		RevealPayloadHash:  hashToBase64URL(payloadHash),
		RevealPayloadHashH: hex.EncodeToString(payloadHash),
	}), nil
}

func (c *VoteLedgerContract) ComputeRevealPayloadHash(ctx contractapi.TransactionContextInterface, candidateID, h, sPrime string) (string, error) {
	if err := requireNonEmpty(candidateID, "candidateId"); err != nil {
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
	hash := revealPayloadDigest(candidateID, hBytes, sPrimeBytes)
	return mustJSON(PayloadHashView{
		CandidateID:          candidateID,
		RevealPayloadHash:    hashToBase64URL(hash),
		RevealPayloadHashHex: hex.EncodeToString(hash),
		HashDefinition:       "sha256('reveal-v1' || uint32be(len(candidateId)) || candidateId || h32 || sPrime32)",
	}), nil
}
