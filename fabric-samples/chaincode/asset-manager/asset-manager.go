package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SmartContract provides functions for managing an Asset
type SmartContract struct {
	contractapi.Contract
}

// Asset describes the structure of your financial accounts
// We use json tags to control how it's serialized
type Asset struct {
	DEALERID    string  `json:"DEALERID"`
	MSISDN      string  `json:"MSISDN"`
	MPIN        string  `json:"MPIN"` // Note: Storing raw PINs is bad practice, but follows the assignment
	BALANCE     float64 `json:"BALANCE"`
	STATUS      string  `json:"STATUS"`
	TRANSAMOUNT float64 `json:"TRANSAMOUNT"`
	TRANSTYPE   string  `json:"TRANSTYPE"`
	REMARKS     string  `json:"REMARKS"`
}

// HistoryQueryResult structure used for returning history query results
type HistoryQueryResult struct {
	Record    *Asset    `json:"record"`
	TxId      string    `json:"txId"`
	Timestamp time.Time `json:"timestamp"`
	IsDelete  bool      `json:"isDelete"`
}

// CreateAsset issues a new asset to the world state.
// The DEALERID will be used as the key.
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface,
	dealerID string, msisdn string, mpin string, balance float64, status string,
	transAmount float64, transType string, remarks string) error {

	exists, err := s.AssetExists(ctx, dealerID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the asset %s already exists", dealerID)
	}

	asset := Asset{
		DEALERID:    dealerID,
		MSISDN:      msisdn,
		MPIN:        mpin,
		BALANCE:     balance,
		STATUS:      status,
		TRANSAMOUNT: transAmount,
		TRANSTYPE:   transType,
		REMARKS:     remarks,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(dealerID, assetJSON)
}

// ReadAsset returns the asset stored in the world state with given id
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, dealerID string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(dealerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", dealerID)
	}

	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, err
	}

	return &asset, nil
}

// UpdateAsset updates an existing asset in the world state
// This is a simple implementation that overwrites the entire asset.
// A real-world app might only update specific fields (e.g., BALANCE).
func (s *SmartContract) UpdateAsset(ctx contractapi.TransactionContextInterface,
	dealerID string, msisdn string, mpin string, balance float64, status string,
	transAmount float64, transType string, remarks string) error {

	exists, err := s.AssetExists(ctx, dealerID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", dealerID)
	}

	// Overwriting original asset with new asset
	asset := Asset{
		DEALERID:    dealerID,
		MSISDN:      msisdn,
		MPIN:        mpin,
		BALANCE:     balance,
		STATUS:      status,
		TRANSAMOUNT: transAmount,
		TRANSTYPE:   transType,
		REMARKS:     remarks,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(dealerID, assetJSON)
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, dealerID string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(dealerID)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}

	return assetJSON != nil, nil
}

// GetAssetHistory returns the chain of custody for an asset
func (s *SmartContract) GetAssetHistory(ctx contractapi.TransactionContextInterface, dealerID string) ([]HistoryQueryResult, error) {
	log.Printf("GetAssetHistory: ID %s", dealerID)

	resultsIterator, err := ctx.GetStub().GetHistoryForKey(dealerID)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var records []HistoryQueryResult
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var asset Asset
		if len(response.Value) > 0 {
			err = json.Unmarshal(response.Value, &asset)
			if err != nil {
				return nil, err
			}
		} else {
			asset = Asset{
				DEALERID: dealerID,
			}
		}

		records = append(records, HistoryQueryResult{
			TxId:      response.TxId,
			Timestamp: response.Timestamp.AsTime(),
			Record:    &asset,
			IsDelete:  response.IsDelete,
		})
	}

	return records, nil
}

func main() {
	assetChaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		log.Panicf("Error creating asset-manager chaincode: %v", err)
	}

	if err := assetChaincode.Start(); err != nil {
		log.Panicf("Error starting asset-manager chaincode: %v", err)
	}
}
