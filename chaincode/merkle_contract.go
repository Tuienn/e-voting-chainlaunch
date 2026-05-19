package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func (c *VoteLedgerContract) CommitMerkleRoot(ctx contractapi.TransactionContextInterface, electionID, merkleRoot, voteCountStr string) (string, error) {
	if err := requireNonEmpty(electionID, "electionId"); err != nil {
		return "", err
	}
	root, err := parseHash32(merkleRoot, "merkleRoot")
	if err != nil {
		return "", err
	}
	voteCount, err := parseUint(voteCountStr, "voteCount")
	if err != nil {
		return "", err
	}

	stats, err := loadStats(ctx, electionID)
	if err != nil {
		return "", err
	}
	if stats.TotalVoteCount != voteCount {
		return "", fmt.Errorf("voteCount mismatch: stats=%d arg=%d", stats.TotalVoteCount, voteCount)
	}

	key, err := compositeKey(ctx, rootPrefix, electionID)
	if err != nil {
		return "", err
	}
	existing, err := ctx.GetStub().GetState(key)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", fmt.Errorf("Merkle root for election %s already exists", electionID)
	}

	record := MerkleRootView{
		DocType:           "root",
		ElectionID:        electionID,
		MerkleRoot:        hex.EncodeToString(root),
		VoteCount:         voteCount,
		Committed:         true,
		ClosedAtTxSeconds: txTimestampSeconds(ctx),
		CommitTxID:        ctx.GetStub().GetTxID(),
	}
	if err := ctx.GetStub().PutState(key, encodeMerkleRoot(record)); err != nil {
		return "", err
	}
	return mustJSON(record), nil
}

func (c *VoteLedgerContract) GetMerkleRoot(ctx contractapi.TransactionContextInterface, electionID string) (string, error) {
	root, err := loadMerkleRoot(ctx, electionID)
	if err != nil {
		return "", err
	}
	if !root.Committed {
		return "", fmt.Errorf("Merkle root for election %s does not exist", electionID)
	}
	return mustJSON(root), nil
}

func (c *VoteLedgerContract) VerifyVoteReceipt(ctx contractapi.TransactionContextInterface, electionID, blindedCommitment, proofJSON string) (string, error) {
	// Validate format nhưng dùng chính chuỗi hex (lowercase) làm input cho hash.
	// Off-chain (libs/fabric/src/lib/merketree/index.ts) build leaf bằng
	// sha256(utf8(commitment_hex_string)), nên on-chain phải băm cùng cách.
	if _, err := parseHash32(blindedCommitment, "blindedCommitment"); err != nil {
		return "", err
	}
	leaf := sha256Bytes([]byte(blindedCommitment))

	out := ReceiptVerifyView{
		ElectionID:    electionID,
		RootCommitted: false,
		InElection:    false,
	}

	root, err := loadMerkleRoot(ctx, electionID)
	if err != nil {
		return "", err
	}
	out.RootCommitted = root.Committed
	if !root.Committed {
		return mustJSON(out), nil
	}
	out.MerkleRoot = root.MerkleRoot
	out.VoteCountOnChain = root.VoteCount

	proof, err := parseProof(proofJSON)
	if err != nil {
		out.ProofError = err.Error()
		return mustJSON(out), nil
	}
	computedRoot := applyProof(leaf, proof)
	rootBytes, err := parseHash32(root.MerkleRoot, "merkleRoot")
	if err != nil {
		return "", err
	}
	out.InElection = bytes.Equal(computedRoot, rootBytes)
	return mustJSON(out), nil
}

func (c *VoteLedgerContract) GetAuditCounts(ctx contractapi.TransactionContextInterface, electionID string) (string, error) {
	stats, err := loadStats(ctx, electionID)
	if err != nil {
		return "", err
	}
	root, err := loadMerkleRoot(ctx, electionID)
	if err != nil {
		return "", err
	}
	return mustJSON(AuditCountsView{
		ElectionID:      electionID,
		TotalVoteCount:  stats.TotalVoteCount,
		RevealCount:     stats.RevealCount,
		RootCommitted:   root.Committed,
		RevealVoteMatch: stats.TotalVoteCount == stats.RevealCount,
	}), nil
}
