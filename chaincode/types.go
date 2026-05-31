package main

import "github.com/hyperledger/fabric-contract-api-go/contractapi"

type VoteLedgerContract struct {
	contractapi.Contract
}

type VoteView struct {
	DocType            string `json:"docType"`
	ElectionID         string `json:"electionId"`
	VoteID             string `json:"voteId"`
	BlindedCommitment  string `json:"blindedCommitment"`
	TxTimestampSeconds string `json:"txTimestamp"`
	TxID               string `json:"txId"`
}

type MerkleRootView struct {
	DocType           string `json:"docType,omitempty"`
	ElectionID        string `json:"electionId"`
	MerkleRoot        string `json:"merkleRoot,omitempty"`
	VoteCount         uint64 `json:"voteCount,omitempty"`
	Committed         bool   `json:"committed"`
	ClosedAtTxSeconds string `json:"closedAt,omitempty"`
	CommitTxID        string `json:"txId,omitempty"`
}

type Stats struct {
	TotalVoteCount uint64 `json:"totalVoteCount"`
	RevealCount    uint64 `json:"revealCount"`
}

type TallyView struct {
	ElectionID string            `json:"electionId"`
	Tally      map[string]uint64 `json:"tally"`
}

type AuditCountsView struct {
	ElectionID      string `json:"electionId"`
	TotalVoteCount  uint64 `json:"totalVoteCount"`
	RevealCount     uint64 `json:"revealCount"`
	RootCommitted   bool   `json:"rootCommitted"`
	RevealVoteMatch bool   `json:"revealVoteMatch"`
}

type UsedRevealView struct {
	ElectionID         string   `json:"electionId"`
	CandidateIDs       []string `json:"candidateIds"`
	RevealKey          string   `json:"revealKey"`
	RevealKeyHex       string   `json:"revealKeyHex"`
	RevealPayloadHash  string   `json:"revealPayloadHash"`
	RevealPayloadHashH string   `json:"revealPayloadHashHex"`
}

type ReceiptVerifyView struct {
	ElectionID       string `json:"electionId"`
	RootCommitted    bool   `json:"rootCommitted"`
	InElection       bool   `json:"inElection"`
	MerkleRoot       string `json:"merkleRoot,omitempty"`
	ProofError       string `json:"proofError,omitempty"`
	VoteCountOnChain uint64 `json:"voteCountOnChain,omitempty"`
}

type PayloadHashView struct {
	CandidateIDs         []string `json:"candidateIds"`
	RevealPayloadHash    string   `json:"revealPayloadHash"`
	RevealPayloadHashHex string   `json:"revealPayloadHashHex"`
	HashDefinition       string   `json:"hashDefinition"`
}
