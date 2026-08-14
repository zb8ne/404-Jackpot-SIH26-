// Package chain wraps the CredentialRegistry contract: department identities,
// transaction signing and the verify read path.
package chain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"credreg/backend/internal/contracts"
)

// Doc types, mirroring the contract's constants.
const (
	DocBirthCertificate  uint8 = 1
	DocDrivingLicence    uint8 = 2
	DocDegreeCertificate uint8 = 3
)

// On-chain status values, mirroring the contract's Status enum.
const (
	StatusValid      uint8 = 0
	StatusSuperseded uint8 = 1
	StatusRevoked    uint8 = 2
)

// Department is one of the hardcoded issuers. Keys are Anvil's well-known
// prefunded accounts — fine here, obviously never anywhere real.
type Department struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	DocType    uint8  `json:"docType"`
	Address    string `json:"address"`
	privateKey string
}

var departments = []Department{
	{
		Slug:       "birth",
		Name:       "Birth Registration Dept",
		DocType:    DocBirthCertificate,
		Address:    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		privateKey: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	},
	{
		Slug:       "transport",
		Name:       "Transport Dept",
		DocType:    DocDrivingLicence,
		Address:    "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		privateKey: "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
	},
	{
		Slug:       "education",
		Name:       "Education Dept",
		DocType:    DocDegreeCertificate,
		Address:    "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
		privateKey: "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
	},
}

func Departments() []Department { return departments }

func DepartmentBySlug(slug string) (Department, bool) {
	for _, d := range departments {
		if d.Slug == slug {
			return d, true
		}
	}
	return Department{}, false
}

var docTypeNames = map[uint8]string{
	DocBirthCertificate:  "birth_certificate",
	DocDrivingLicence:    "driving_licence",
	DocDegreeCertificate: "degree_certificate",
}

func DocTypeName(t uint8) string {
	if n, ok := docTypeNames[t]; ok {
		return n
	}
	return "unknown"
}

func DocTypeByName(name string) (uint8, bool) {
	for t, n := range docTypeNames {
		if n == name {
			return t, true
		}
	}
	return 0, false
}

// Client talks to the registry on the local Anvil node.
type Client struct {
	eth      *ethclient.Client
	registry *contracts.CredentialRegistry
	chainID  *big.Int
	Address  common.Address
}

func Dial(ctx context.Context, rpcURL string, contractAddr common.Address) (*Client, error) {
	eth, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	chainID, err := eth.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}
	registry, err := contracts.NewCredentialRegistry(contractAddr, eth)
	if err != nil {
		return nil, fmt.Errorf("bind registry: %w", err)
	}
	return &Client{eth: eth, registry: registry, chainID: chainID, Address: contractAddr}, nil
}

func (c *Client) auth(ctx context.Context, dept Department) (*bind.TransactOpts, error) {
	key, err := crypto.HexToECDSA(dept.privateKey)
	if err != nil {
		return nil, err
	}
	opts, err := bind.NewKeyedTransactorWithChainID(key, c.chainID)
	if err != nil {
		return nil, err
	}
	opts.Context = ctx
	return opts, nil
}

// Record is a credential as read back off chain.
type Record struct {
	Found     bool   `json:"found"`
	DocID     string `json:"docId"`
	DocType   uint8  `json:"docType"`
	Issuer    string `json:"issuer"`
	Timestamp int64  `json:"timestamp"`
	Status    uint8  `json:"status"`
	PrevHash  string `json:"prevHash"`
}

func (c *Client) Verify(ctx context.Context, docHash [32]byte) (Record, error) {
	res, err := c.registry.Verify(&bind.CallOpts{Context: ctx}, docHash)
	if err != nil {
		return Record{}, err
	}
	if !res.Found {
		return Record{Found: false}, nil
	}
	return Record{
		Found:     true,
		DocID:     res.Cred.DocId,
		DocType:   res.Cred.DocType,
		Issuer:    res.Cred.Issuer.Hex(),
		Timestamp: int64(res.Cred.Timestamp),
		Status:    res.Cred.Status,
		PrevHash:  common.BytesToHash(res.Cred.PrevHash[:]).Hex(),
	}, nil
}

// CurrentHashOf resolves a docId to the hash a verifier should expect today.
// A zero hash means the registry has never issued that id.
func (c *Client) CurrentHashOf(ctx context.Context, docID string) ([32]byte, bool, error) {
	h, err := c.registry.CurrentHashOf(&bind.CallOpts{Context: ctx}, docID)
	if err != nil {
		return h, false, err
	}
	return h, h != [32]byte{}, nil
}

func (c *Client) Issue(ctx context.Context, dept Department, docHash [32]byte, docID string, docType uint8) (string, error) {
	opts, err := c.auth(ctx, dept)
	if err != nil {
		return "", err
	}
	tx, err := c.registry.Issue(opts, docHash, docID, docType)
	if err != nil {
		return "", unwrapRevert(err)
	}
	return c.wait(ctx, tx)
}

func (c *Client) Supersede(ctx context.Context, dept Department, oldHash, newHash [32]byte, docID string, docType uint8) (string, error) {
	opts, err := c.auth(ctx, dept)
	if err != nil {
		return "", err
	}
	tx, err := c.registry.Supersede(opts, oldHash, newHash, docID, docType)
	if err != nil {
		return "", unwrapRevert(err)
	}
	return c.wait(ctx, tx)
}

func (c *Client) Revoke(ctx context.Context, dept Department, docHash [32]byte, docType uint8) (string, error) {
	opts, err := c.auth(ctx, dept)
	if err != nil {
		return "", err
	}
	tx, err := c.registry.Revoke(opts, docHash, docType)
	if err != nil {
		return "", unwrapRevert(err)
	}
	return c.wait(ctx, tx)
}

// wait blocks until the tx is mined. Anvil mines instantly, so this is quick.
func (c *Client) wait(ctx context.Context, tx *types.Transaction) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, c.eth, tx)
	if err != nil {
		return "", err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return "", fmt.Errorf("transaction reverted: %s", tx.Hash().Hex())
	}
	return tx.Hash().Hex(), nil
}

// unwrapRevert turns go-ethereum's noisy gas-estimation error into something a
// demo audience can read. The role check is the one people will actually trip.
func unwrapRevert(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "NotAuthorizedForDocType") ||
		strings.Contains(msg, "execution reverted") && strings.Contains(msg, "0x") {
		return fmt.Errorf("contract rejected the call (department not authorised for this document type): %w", err)
	}
	return err
}
