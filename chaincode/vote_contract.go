package main

import (
	"encoding/hex"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func (c *VoteLedgerContract) SubmitVote(ctx contractapi.TransactionContextInterface, electionID, voteID, blindedCommitment string) (string, error) {
	if err := requireNonEmpty(electionID, "electionId"); err != nil {
		return "", err
	}
	if err := requireNonEmpty(voteID, "voteId"); err != nil {
		return "", err
	}
	commitment, err := parseHash32(blindedCommitment, "blindedCommitment")
	if err != nil {
		return "", err
	}

	root, err := loadMerkleRoot(ctx, electionID)
	if err != nil {
		return "", err
	}
	if root.Committed {
		return "", fmt.Errorf("election %s already has Merkle root committed", electionID)
	}

	key, err := compositeKey(ctx, votePrefix, electionID, voteID)
	if err != nil {
		return "", err
	}
	existing, err := ctx.GetStub().GetState(key)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", fmt.Errorf("vote %s for election %s already exists", voteID, electionID)
	}

	record := VoteView{
		DocType:            "vote",
		ElectionID:         electionID,
		VoteID:             voteID,
		BlindedCommitment:  hex.EncodeToString(commitment),
		TxTimestampSeconds: txTimestampSeconds(ctx),
		TxID:               ctx.GetStub().GetTxID(),
	}
	if err := ctx.GetStub().PutState(key, encodeVote(record)); err != nil {
		return "", err
	}

	stats, err := loadStats(ctx, electionID)
	if err != nil {
		return "", err
	}
	stats.TotalVoteCount++
	if err := saveStats(ctx, electionID, stats); err != nil {
		return "", err
	}

	return mustJSON(record), nil
}

func (c *VoteLedgerContract) GetVote(ctx contractapi.TransactionContextInterface, electionID, voteID string) (string, error) {
	key, err := compositeKey(ctx, votePrefix, electionID, voteID)
	if err != nil {
		return "", err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("vote %s for election %s does not exist", voteID, electionID)
	}
	record, err := decodeVote(data)
	if err != nil {
		return "", err
	}
	return mustJSON(record), nil
}
