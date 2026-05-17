package main

import (
	"strconv"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func txTimestampSeconds(ctx contractapi.TransactionContextInterface) string {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil || ts == nil {
		return ""
	}
	return strconv.FormatInt(ts.Seconds, 10)
}
