package main

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Configuration for our API
// These paths are relative to the 'test-network' directory
const (
	mspID         = "Org1MSP"
	cryptoPath    = "organizations/peerOrganizations/org1.example.com"
	certPath      = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem"
	keyPath       = cryptoPath + "/users/User1@org1.example.com/msp/keystore/" // Will find the first key
	tlsCertPath   = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"
	peerEndpoint  = "localhost:7051"
	gatewayPeer   = "peer0.org1.example.com"
	channelName   = "mychannel"
	chaincodeName = "asset-manager"
)

// Main function: sets up the API server
func main() {
	log.Println("Starting Asset Manager API server...")

	// Set up the gRPC connection to the Fabric peer
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	// Create the Fabric Gateway client
	gw := newGateway(clientConnection)
	defer gw.Close()

	// Get the network (channel)
	network := gw.GetNetwork(channelName)

	// Create an 'ApiHandler' struct that holds our contract object
	apiHandler := &ApiHandler{
		Contract: network.GetContract(chaincodeName),
	}

	// Set up the web server routes
	r := mux.NewRouter()
	r.HandleFunc("/api/assets", apiHandler.CreateAssetHandler).Methods("POST")
	r.HandleFunc("/api/assets/{id}", apiHandler.ReadAssetHandler).Methods("GET")
	r.HandleFunc("/api/assets/history/{id}", apiHandler.GetAssetHistoryHandler).Methods("GET")
	// Add more routes here for Update, History, etc.

	log.Println("Server is listening on http://localhost:8080")
	// Start the server
	log.Fatal(http.ListenAndServe(":8080", r))
}

// ApiHandler holds the contract object
type ApiHandler struct {
	Contract *client.Contract
}

// CreateAssetHandler handles POST /api/assets
// It reads JSON from the request body to create an asset
func (h *ApiHandler) CreateAssetHandler(w http.ResponseWriter, r *http.Request) {
	// Define a temporary struct to capture the incoming JSON
	// This matches the fields in your 'Asset' struct in the chaincode
	var asset struct {
		DEALERID    string `json:"DEALERID"`
		MSISDN      string `json:"MSISDN"`
		MPIN        string `json:"MPIN"`
		BALANCE     string `json:"BALANCE"` // Receive as string for simplicity
		STATUS      string `json:"STATUS"`
		TRANSAMOUNT string `json:"TRANSAMOUNT"` // Receive as string
		TRANSTYPE   string `json:"TRANSTYPE"`
		REMARKS     string `json:"REMARKS"`
	}

	// Decode the JSON request body into our struct
	if err := json.NewDecoder(r.Body).Decode(&asset); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call the 'CreateAsset' function in our smart contract
	log.Printf("--> Submitting Transaction: CreateAsset, ID: %s", asset.DEALERID)
	_, err := h.Contract.SubmitTransaction("CreateAsset",
		asset.DEALERID,
		asset.MSISDN,
		asset.MPIN,
		asset.BALANCE,
		asset.STATUS,
		asset.TRANSAMOUNT,
		asset.TRANSTYPE,
		asset.REMARKS,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit transaction: %s", err), http.StatusInternalServerError)
		return
	}

	log.Printf("<-- Transaction Committed: CreateAsset, ID: %s", asset.DEALERID)
	// Send a success response
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Asset created successfully"})
}

// ReadAssetHandler handles GET /api/assets/{id}
// It reads the asset ID from the URL path
func (h *ApiHandler) ReadAssetHandler(w http.ResponseWriter, r *http.Request) {
	// Get the 'id' variable from the URL
	vars := mux.Vars(r)
	assetID := vars["id"]

	// Call the 'ReadAsset' function in our smart contract
	log.Printf("--> Evaluating Transaction: ReadAsset, ID: %s", assetID)
	result, err := h.Contract.EvaluateTransaction("ReadAsset", assetID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to evaluate transaction: %s", err), http.StatusInternalServerError)
		return
	}
	log.Printf("<-- Transaction Evaluated: ReadAsset, ID: %s", assetID)

	// Send the result back as JSON
	w.Header().Set("Content-Type", "application/json")
	// The result from the chaincode is raw JSON, so we can write it directly
	w.Write(result)
}

// GetAssetHistoryHandler handles GET /api/assets/history/{id}
func (h *ApiHandler) GetAssetHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["id"]

	log.Printf("--> Evaluating Transaction: GetAssetHistory, ID: %s", assetID)
	result, err := h.Contract.EvaluateTransaction("GetAssetHistory", assetID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to evaluate transaction: %s", err), http.StatusInternalServerError)
		return
	}
	log.Printf("<-- Transaction Evaluated: GetAssetHistory, ID: %s", assetID)

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// --- Helper Functions for Fabric Connection ---

// newGrpcConnection creates a gRPC connection to the peer
func newGrpcConnection() *grpc.ClientConn {
	// We need to use the full path relative to the /workspaces/ directory
	// We assume the API is running from 'fabric-samples/asset-manager-api'
	// So we go up one level and into 'test-network'
	peerCert, err := os.ReadFile("../test-network/" + tlsCertPath)
	if err != nil {
		panic(fmt.Errorf("failed to load peer TLS certificate: %w", err))
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(peerCert) {
		panic("failed to add peer certificate to pool")
	}

	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)
	conn, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}
	return conn
}

// newGateway creates a new Gateway client
func newGateway(conn *grpc.ClientConn) *client.Gateway {
	id := newIdentity()
	sign := newSign()

	// ***** THIS IS THE FIX *****
	// The first argument must be the identity, followed by options.
	gw, err := client.Connect(
		id, // 1. The identity (as the first argument)
		// 2. All other items as options
		client.WithClientConnection(conn),
		client.WithSign(sign),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	// ***************************

	if err != nil {
		panic(fmt.Errorf("failed to connect to Gateway: %w", err))
	}
	return gw
}

// newIdentity creates a client identity for connecting to the Gateway
func newIdentity() *identity.X509Identity {
	// We need to use the full path relative to the /workspaces/ directory
	// We assume the API is running from 'fabric-samples/asset-manager-api'
	// So we go up one level and into 'test-network'
	certData, err := os.ReadFile("../test-network/" + certPath)
	if err != nil {
		panic(fmt.Errorf("failed to read certificate file: %w", err))
	}

	cert, err := identity.CertificateFromPEM(certData)
	if err != nil {
		panic(err)
	}

	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		panic(err)
	}
	return id
}

// newSign creates a function that signs transactions
func newSign() identity.Sign {
	// We need to use the full path relative to the /workspaces/ directory
	// We assume the API is running from 'fabric-samples/asset-manager-api'
	// So we go up one level and into 'test-network'

	// The key file has a random name, so we read the directory
	files, err := os.ReadDir("../test-network/" + keyPath)
	if err != nil {
		panic(fmt.Errorf("failed to read private key directory: %w", err))
	}
	if len(files) == 0 {
		panic("no private key found in directory")
	}
	// Use the first key found
	keyFileData, err := os.ReadFile(path.Join("../test-network/"+keyPath, files[0].Name()))
	if err != nil {
		panic(fmt.Errorf("failed to read private key file: %w", err))
	}

	key, err := identity.PrivateKeyFromPEM(keyFileData)
	if err != nil {
		panic(err)
	}

	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		panic(err)
	}
	return sign
}
