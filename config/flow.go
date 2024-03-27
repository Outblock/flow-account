package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onflow/cadence"
	c_json "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"google.golang.org/grpc"
)

type Wallet struct {
	Address string `json:"address"`
	Key     string `json:"key"`
}

type Identifier [32]byte

func CreateFlowKey(hashAlgo string, signAlgo string, publicKey string, weight int, network string, c *gin.Context) *flow.Transaction {

	node := Flow(network)

	serviceAddressHex, servicePrivKeyHex, keyIndex := GetKey(network, c)
	serviceSigAlgoHex := "ECDSA_P256"

	// fmt.Printf("Service private key %s\n", servicePrivKeyHex)

	gasLimit := uint64(100)

	tx := CreateAccount(node, publicKey, signAlgo, hashAlgo, serviceAddressHex, servicePrivKeyHex, serviceSigAlgoHex, gasLimit, keyIndex, weight, network)

	return tx
}

func CreateAccount(node string,
	publicKeyHex string,
	sigAlgoName string,
	hashAlgoName string,
	serviceAddressHex string,
	servicePrivKeyHex string,
	serviceSigAlgoName string,
	gasLimit uint64,
	keyIndex int64,
	weight int,
	network string) *flow.Transaction {

	ctx := context.Background()

	sigAlgo := crypto.StringToSignatureAlgorithm(sigAlgoName)
	publicKey, err := crypto.DecodePublicKeyHex(sigAlgo, publicKeyHex)
	if err != nil {
		log.Println("failed to decode public key hex: ", err)
	}
	// if err != nil {
	// 	log.Println("Decode Public Key Hex Error: ", err)
	// }

	hashAlgo := crypto.StringToHashAlgorithm(hashAlgoName)

	accountKey := flow.NewAccountKey().
		SetPublicKey(publicKey).
		SetSigAlgo(sigAlgo).
		SetHashAlgo(hashAlgo).
		SetWeight(weight)

	c, err := client.New(node, grpc.WithInsecure())
	if err != nil {
		log.Println("failed to connect to node")
	}

	serviceSigAlgo := crypto.StringToSignatureAlgorithm(serviceSigAlgoName)
	servicePrivKey, err := crypto.DecodePrivateKeyHex(serviceSigAlgo, servicePrivKeyHex)
	if err != nil {
		log.Println(err)
	}

	serviceAddress := flow.HexToAddress(serviceAddressHex)
	//serviceAddress := flow.HexToAddress(serviceAddressHex)
	serviceAccount, err := c.GetAccountAtLatestBlock(ctx, serviceAddress)
	if err != nil {
		log.Println(err)
	}

	serviceAccountKey := serviceAccount.Keys[keyIndex]
	serviceSigner, err := crypto.NewInMemorySigner(servicePrivKey, serviceAccountKey.HashAlgo)
	if err != nil {
		log.Println("Service Sign Error: ", err)

	}

	var tx *flow.Transaction
	if network == "previewnet" {
		tx, err = CreatePreviewnetAccount([]*flow.AccountKey{accountKey}, nil, serviceAddress)
	} else {
		tx, err = templates.CreateAccount([]*flow.AccountKey{accountKey}, nil, serviceAddress)
	}

	// tx, err := templates.CreateAccount([]*flow.AccountKey{accountKey}, nil, serviceAddress)
	var transactionScript string
	if network == "testnet" {
		transactionScript = "import Crypto\n import FlowToken from 0x7e60df042a9c0868\n import FungibleToken from 0x9a0766d93b6608b7\n transaction(publicKeys: [Crypto.KeyListEntry], contracts: {String: String}, fundAmount: UFix64) {\n let tokenReceiver: &{FungibleToken.Receiver}\n let sentVault: @FungibleToken.Vault\n prepare(signer: AuthAccount) {\n let account = AuthAccount(payer: signer)\n for key in publicKeys {\n account.keys.add(publicKey: key.publicKey, hashAlgorithm: key.hashAlgorithm, weight: key.weight)\n }\n for contract in contracts.keys {\n account.contracts.add(name: contract, code: contracts[contract]!.decodeHex())\n }\n self.tokenReceiver = account.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>() ?? panic(\"Unable to borrow receiver reference\")\n let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault) ?? panic(\"Could not borrow reference to the owner''s Vault!\")\n self.sentVault <- vaultRef.withdraw(amount: fundAmount)\n }\n execute {\n self.tokenReceiver.deposit(from: <-self.sentVault)\n }\n }\n "
	} else if network == "previewnet" {
		transactionScript = "import Crypto\n transaction(publicKeys: [Crypto.KeyListEntry], contracts: {String: String}) {\n prepare(signer: auth(BorrowValue | Storage) &Account) {\n let account = Account(payer: signer)\n for key in publicKeys {\n account.keys.add(publicKey: key.publicKey, hashAlgorithm: key.hashAlgorithm, weight: key.weight)\n }\n for contract in contracts.keys {\n account.contracts.add(name: contract, code: contracts[contract]!.decodeHex())\n }\n }\n }"
	} else {
		transactionScript = "import Crypto\n import FlowToken from 0x1654653399040a61\n import FungibleToken from 0xf233dcee88fe0abe\n transaction(publicKeys: [Crypto.KeyListEntry], contracts: {String: String}, fundAmount: UFix64) {\n let tokenReceiver: &{FungibleToken.Receiver}\n let sentVault: @FungibleToken.Vault\n prepare(signer: AuthAccount) {\n let account = AuthAccount(payer: signer)\n for key in publicKeys {\n account.keys.add(publicKey: key.publicKey, hashAlgorithm: key.hashAlgorithm, weight: key.weight)\n }\n for contract in contracts.keys {\n account.contracts.add(name: contract, code: contracts[contract]!.decodeHex())\n }\n self.tokenReceiver = account.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>() ?? panic(\"Unable to borrow receiver reference\")\n let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault) ?? panic(\"Could not borrow reference to the owner''s Vault!\")\n self.sentVault <- vaultRef.withdraw(amount: fundAmount)\n }\n execute {\n self.tokenReceiver.deposit(from: <-self.sentVault)\n }\n }\n "
	}

	s := map[string]interface{}{}
	s["type"] = "UFix64"
	s["value"] = "0.00100000"

	cv, err := ArgAsCadence(s)
	if err != nil {
		log.Println(err)
	}

	err = tx.AddArgument(cv)
	if err != nil {
		log.Println(err)
	}

	tx.SetScript([]byte(transactionScript))

	if err != nil {
		log.Println("Create Account Error: ", err)
	}
	tx.SetProposalKey(serviceAddress, serviceAccountKey.Index, serviceAccountKey.SequenceNumber)
	tx.SetPayer(serviceAddress)
	tx.SetGasLimit(uint64(gasLimit))

	latestBlock, err := c.GetLatestBlockHeader(context.Background(), true)
	tx.SetReferenceBlockID(latestBlock.ID)

	err = tx.SignEnvelope(serviceAddress, serviceAccountKey.Index, serviceSigner)
	err = c.SendTransaction(context.Background(), *tx)
	if err != nil {
		log.Println(err)
	}
	return tx
}

func WaitForSeal(id flow.Identifier, network string) string {
	ctx := context.Background()

	node := Flow(network)
	c, connectErr := client.New(node, grpc.WithInsecure())
	if connectErr != nil {
		log.Println("failed to connect to node")
	}

	result, err := c.GetTransactionResult(ctx, id)
	Handle(err)

	fmt.Printf("Waiting for transaction %s to be sealed...\n", id)

	for result.Status != flow.TransactionStatusSealed {
		time.Sleep(time.Second)
		fmt.Print(".")
		result, err = c.GetTransactionResult(ctx, id)
		Handle(err)
	}

	fmt.Printf("Transaction %s sealed\n", id)
	newAddress := findAccount(ctx, c, id)
	addressString := flow.Address.String(newAddress)
	return addressString
}

func findAccount(ctx context.Context, c *client.Client, id flow.Identifier) flow.Address {

	result, err := c.GetTransactionResult(ctx, id)
	if err != nil {
		log.Println("failed to get transaction result")
	}

	var newAddress flow.Address

	if result.Status != flow.TransactionStatusSealed {
		log.Println("address not known until transaction is sealed")
	}

	for _, event := range result.Events {
		if event.Type == flow.EventAccountCreated {
			newAddress = flow.AccountCreatedEvent(event).Address()
			break
		}
	}

	fmt.Println()
	fmt.Printf("New address -> %s", newAddress)
	return newAddress
}

func Handle(err error) {

	if err != nil {
		// fmt.Println("err:", err.Error())
		log.Println(err)
	}
}

func DecodePublicKey(publicKey string) (crypto.PublicKey, error) {
	pk, error := crypto.DecodePublicKeyHex(crypto.ECDSA_secp256k1, publicKey)

	return pk, error
}

func SendTransaction() {
	flow.NewTransaction()
}

func ArgAsCadence(a interface{}) (cadence.Value, error) {
	c, ok := a.(cadence.Value)
	if ok {
		return c, nil
	}
	// Convert to json bytes so we can use cadence's own encoding library
	j, err := json.Marshal(a)
	if err != nil {
		return cadence.Void{}, err
	}
	// Use cadence's own encoding library
	c, err = c_json.Decode(j)

	if err != nil {
		return cadence.Void{}, err
	}
	return c, nil
}
