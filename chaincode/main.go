package main

import (
	"log"
	"os"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	chaincode, err := contractapi.NewChaincode(new(VoteLedgerContract))
	if err != nil {
		log.Panicf("create vote ledger chaincode failed: %v", err)
	}

	ccid := os.Getenv("CHAINCODE_ID")
	if ccid == "" {
		log.Panic("CHAINCODE_ID is required")
	}

	address := os.Getenv("CHAINCODE_SERVER_ADDRESS")
	if address == "" {
		address = "0.0.0.0:9999"
	}

	server := &shim.ChaincodeServer{
		CCID:    ccid,
		Address: address,
		CC:      chaincode,
		TLSProps: shim.TLSProperties{
			Disabled: true,
		},
	}

	log.Printf("starting vote ledger chaincode server at %s", address)

	if err := server.Start(); err != nil {
		log.Panicf("start vote ledger chaincode server failed: %v", err)
	}
}
