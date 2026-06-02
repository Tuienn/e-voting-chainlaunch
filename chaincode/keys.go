package main

import "github.com/hyperledger/fabric-contract-api-go/contractapi"

const (
	votePrefix       = "vote"
	rootPrefix       = "root"
	usedRevealPrefix = "usedReveal"
)

func compositeKey(ctx contractapi.TransactionContextInterface, objectType string, attrs ...string) (string, error) {
	return ctx.GetStub().CreateCompositeKey(objectType, attrs)
}
