package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

type ProofStep struct {
	Position string `json:"position"`
	Hash     string `json:"hash"`
}

func hashPair(left []byte, right []byte) []byte {
	sum := sha256.Sum256(append(append([]byte(nil), left...), right...))
	return sum[:]
}

func parseProof(value string) ([]ProofStep, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var proof []ProofStep
	if err := json.Unmarshal([]byte(value), &proof); err != nil {
		return nil, fmt.Errorf("proof must be JSON array: %w", err)
	}
	return proof, nil
}

func applyProof(leaf []byte, proof []ProofStep) ([]byte, error) {
	current := append([]byte(nil), leaf...)
	for i, step := range proof {
		sibling, err := parseHash32(step.Hash, fmt.Sprintf("proof[%d].hash", i))
		if err != nil {
			return nil, err
		}
		switch step.Position {
		case "left":
			current = hashPair(sibling, current)
		case "right":
			current = hashPair(current, sibling)
		default:
			return nil, fmt.Errorf("proof[%d].position must be left or right", i)
		}
	}
	return current, nil
}
