// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
	_ = time.Tick
	_ = context.Background
)

// CredentialRegistryCredential is an auto generated low-level Go binding around an user-defined struct.
type CredentialRegistryCredential struct {
	DocId     string
	DocType   uint8
	Issuer    common.Address
	Timestamp uint64
	Status    uint8
	PrevHash  [32]byte
}

// CredentialRegistryMetaData contains all meta data concerning the CredentialRegistry contract.
var CredentialRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"DOC_BIRTH_CERTIFICATE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"DOC_DEGREE_CERTIFICATE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"DOC_DRIVING_LICENCE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"DOC_NONE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"admin\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"currentHashOf\",\"inputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"grantRole\",\"inputs\":[{\"name\":\"dept\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"docType\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"issue\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"docId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"docType\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"issuerRole\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"revoke\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"docType\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"supersede\",\"inputs\":[{\"name\":\"oldHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"newHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"docId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"docType\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"verify\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"found\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"cred\",\"type\":\"tuple\",\"internalType\":\"structCredentialRegistry.Credential\",\"components\":[{\"name\":\"docId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"docType\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"issuer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timestamp\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumCredentialRegistry.Status\"},{\"name\":\"prevHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]}],\"stateMutability\":\"view\"},{\"type\":\"event\",\"name\":\"Issued\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"docId\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"docType\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"uint8\"},{\"name\":\"issuer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Revoked\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleGranted\",\"inputs\":[{\"name\":\"dept\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"docType\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"uint8\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Superseded\",\"inputs\":[{\"name\":\"oldHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"newHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AlreadyIssued\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"DocIdInUse\",\"inputs\":[{\"name\":\"docId\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"InvalidDocType\",\"inputs\":[{\"name\":\"docType\",\"type\":\"uint8\",\"internalType\":\"uint8\"}]},{\"type\":\"error\",\"name\":\"NotAdmin\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotAuthorizedForDocType\",\"inputs\":[{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"required\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"held\",\"type\":\"uint8\",\"internalType\":\"uint8\"}]},{\"type\":\"error\",\"name\":\"NotIssuer\",\"inputs\":[{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"NotValid\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"UnknownCredential\",\"inputs\":[{\"name\":\"docHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]}]",
}

// CredentialRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use CredentialRegistryMetaData.ABI instead.
var CredentialRegistryABI = CredentialRegistryMetaData.ABI

// CredentialRegistry is an auto generated Go binding around an Ethereum contract.
type CredentialRegistry struct {
	CredentialRegistryCaller     // Read-only binding to the contract
	CredentialRegistryTransactor // Write-only binding to the contract
	CredentialRegistryFilterer   // Log filterer for contract events
}

// CredentialRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type CredentialRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CredentialRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CredentialRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CredentialRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CredentialRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CredentialRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CredentialRegistrySession struct {
	Contract     *CredentialRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// CredentialRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CredentialRegistryCallerSession struct {
	Contract *CredentialRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// CredentialRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CredentialRegistryTransactorSession struct {
	Contract     *CredentialRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// CredentialRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type CredentialRegistryRaw struct {
	Contract *CredentialRegistry // Generic contract binding to access the raw methods on
}

// CredentialRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CredentialRegistryCallerRaw struct {
	Contract *CredentialRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// CredentialRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CredentialRegistryTransactorRaw struct {
	Contract *CredentialRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCredentialRegistry creates a new instance of CredentialRegistry, bound to a specific deployed contract.
func NewCredentialRegistry(address common.Address, backend bind.ContractBackend) (*CredentialRegistry, error) {
	contract, err := bindCredentialRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistry{CredentialRegistryCaller: CredentialRegistryCaller{contract: contract}, CredentialRegistryTransactor: CredentialRegistryTransactor{contract: contract}, CredentialRegistryFilterer: CredentialRegistryFilterer{contract: contract}}, nil
}

// NewCredentialRegistryCaller creates a new read-only instance of CredentialRegistry, bound to a specific deployed contract.
func NewCredentialRegistryCaller(address common.Address, caller bind.ContractCaller) (*CredentialRegistryCaller, error) {
	contract, err := bindCredentialRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistryCaller{contract: contract}, nil
}

// NewCredentialRegistryTransactor creates a new write-only instance of CredentialRegistry, bound to a specific deployed contract.
func NewCredentialRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*CredentialRegistryTransactor, error) {
	contract, err := bindCredentialRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistryTransactor{contract: contract}, nil
}

// NewCredentialRegistryFilterer creates a new log filterer instance of CredentialRegistry, bound to a specific deployed contract.
func NewCredentialRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*CredentialRegistryFilterer, error) {
	contract, err := bindCredentialRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistryFilterer{contract: contract}, nil
}

// bindCredentialRegistry binds a generic wrapper to an already deployed contract.
func bindCredentialRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := CredentialRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CredentialRegistry *CredentialRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CredentialRegistry.Contract.CredentialRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CredentialRegistry *CredentialRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.CredentialRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CredentialRegistry *CredentialRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.CredentialRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CredentialRegistry *CredentialRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CredentialRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CredentialRegistry *CredentialRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CredentialRegistry *CredentialRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.contract.Transact(opts, method, params...)
}

// DOCBIRTHCERTIFICATE is a free data retrieval call binding the contract method 0xc9243318.
//
// Solidity: function DOC_BIRTH_CERTIFICATE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCaller) DOCBIRTHCERTIFICATE(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "DOC_BIRTH_CERTIFICATE")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// DOCBIRTHCERTIFICATE is a free data retrieval call binding the contract method 0xc9243318.
//
// Solidity: function DOC_BIRTH_CERTIFICATE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistrySession) DOCBIRTHCERTIFICATE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCBIRTHCERTIFICATE(&_CredentialRegistry.CallOpts)
}

// DOCBIRTHCERTIFICATE is a free data retrieval call binding the contract method 0xc9243318.
//
// Solidity: function DOC_BIRTH_CERTIFICATE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCallerSession) DOCBIRTHCERTIFICATE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCBIRTHCERTIFICATE(&_CredentialRegistry.CallOpts)
}

// DOCDEGREECERTIFICATE is a free data retrieval call binding the contract method 0x36babbe4.
//
// Solidity: function DOC_DEGREE_CERTIFICATE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCaller) DOCDEGREECERTIFICATE(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "DOC_DEGREE_CERTIFICATE")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// DOCDEGREECERTIFICATE is a free data retrieval call binding the contract method 0x36babbe4.
//
// Solidity: function DOC_DEGREE_CERTIFICATE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistrySession) DOCDEGREECERTIFICATE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCDEGREECERTIFICATE(&_CredentialRegistry.CallOpts)
}

// DOCDEGREECERTIFICATE is a free data retrieval call binding the contract method 0x36babbe4.
//
// Solidity: function DOC_DEGREE_CERTIFICATE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCallerSession) DOCDEGREECERTIFICATE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCDEGREECERTIFICATE(&_CredentialRegistry.CallOpts)
}

// DOCDRIVINGLICENCE is a free data retrieval call binding the contract method 0x83a40337.
//
// Solidity: function DOC_DRIVING_LICENCE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCaller) DOCDRIVINGLICENCE(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "DOC_DRIVING_LICENCE")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// DOCDRIVINGLICENCE is a free data retrieval call binding the contract method 0x83a40337.
//
// Solidity: function DOC_DRIVING_LICENCE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistrySession) DOCDRIVINGLICENCE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCDRIVINGLICENCE(&_CredentialRegistry.CallOpts)
}

// DOCDRIVINGLICENCE is a free data retrieval call binding the contract method 0x83a40337.
//
// Solidity: function DOC_DRIVING_LICENCE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCallerSession) DOCDRIVINGLICENCE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCDRIVINGLICENCE(&_CredentialRegistry.CallOpts)
}

// DOCNONE is a free data retrieval call binding the contract method 0xc9c3d5ec.
//
// Solidity: function DOC_NONE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCaller) DOCNONE(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "DOC_NONE")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// DOCNONE is a free data retrieval call binding the contract method 0xc9c3d5ec.
//
// Solidity: function DOC_NONE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistrySession) DOCNONE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCNONE(&_CredentialRegistry.CallOpts)
}

// DOCNONE is a free data retrieval call binding the contract method 0xc9c3d5ec.
//
// Solidity: function DOC_NONE() view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCallerSession) DOCNONE() (uint8, error) {
	return _CredentialRegistry.Contract.DOCNONE(&_CredentialRegistry.CallOpts)
}

// Admin is a free data retrieval call binding the contract method 0xf851a440.
//
// Solidity: function admin() view returns(address)
func (_CredentialRegistry *CredentialRegistryCaller) Admin(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "admin")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Admin is a free data retrieval call binding the contract method 0xf851a440.
//
// Solidity: function admin() view returns(address)
func (_CredentialRegistry *CredentialRegistrySession) Admin() (common.Address, error) {
	return _CredentialRegistry.Contract.Admin(&_CredentialRegistry.CallOpts)
}

// Admin is a free data retrieval call binding the contract method 0xf851a440.
//
// Solidity: function admin() view returns(address)
func (_CredentialRegistry *CredentialRegistryCallerSession) Admin() (common.Address, error) {
	return _CredentialRegistry.Contract.Admin(&_CredentialRegistry.CallOpts)
}

// CurrentHashOf is a free data retrieval call binding the contract method 0x292e6cbf.
//
// Solidity: function currentHashOf(string ) view returns(bytes32)
func (_CredentialRegistry *CredentialRegistryCaller) CurrentHashOf(opts *bind.CallOpts, arg0 string) ([32]byte, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "currentHashOf", arg0)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// CurrentHashOf is a free data retrieval call binding the contract method 0x292e6cbf.
//
// Solidity: function currentHashOf(string ) view returns(bytes32)
func (_CredentialRegistry *CredentialRegistrySession) CurrentHashOf(arg0 string) ([32]byte, error) {
	return _CredentialRegistry.Contract.CurrentHashOf(&_CredentialRegistry.CallOpts, arg0)
}

// CurrentHashOf is a free data retrieval call binding the contract method 0x292e6cbf.
//
// Solidity: function currentHashOf(string ) view returns(bytes32)
func (_CredentialRegistry *CredentialRegistryCallerSession) CurrentHashOf(arg0 string) ([32]byte, error) {
	return _CredentialRegistry.Contract.CurrentHashOf(&_CredentialRegistry.CallOpts, arg0)
}

// IssuerRole is a free data retrieval call binding the contract method 0x106cf75a.
//
// Solidity: function issuerRole(address ) view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCaller) IssuerRole(opts *bind.CallOpts, arg0 common.Address) (uint8, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "issuerRole", arg0)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// IssuerRole is a free data retrieval call binding the contract method 0x106cf75a.
//
// Solidity: function issuerRole(address ) view returns(uint8)
func (_CredentialRegistry *CredentialRegistrySession) IssuerRole(arg0 common.Address) (uint8, error) {
	return _CredentialRegistry.Contract.IssuerRole(&_CredentialRegistry.CallOpts, arg0)
}

// IssuerRole is a free data retrieval call binding the contract method 0x106cf75a.
//
// Solidity: function issuerRole(address ) view returns(uint8)
func (_CredentialRegistry *CredentialRegistryCallerSession) IssuerRole(arg0 common.Address) (uint8, error) {
	return _CredentialRegistry.Contract.IssuerRole(&_CredentialRegistry.CallOpts, arg0)
}

// Verify is a free data retrieval call binding the contract method 0x75e36616.
//
// Solidity: function verify(bytes32 docHash) view returns(bool found, (string,uint8,address,uint64,uint8,bytes32) cred)
func (_CredentialRegistry *CredentialRegistryCaller) Verify(opts *bind.CallOpts, docHash [32]byte) (struct {
	Found bool
	Cred  CredentialRegistryCredential
}, error) {
	var out []interface{}
	err := _CredentialRegistry.contract.Call(opts, &out, "verify", docHash)

	outstruct := new(struct {
		Found bool
		Cred  CredentialRegistryCredential
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Found = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.Cred = *abi.ConvertType(out[1], new(CredentialRegistryCredential)).(*CredentialRegistryCredential)

	return *outstruct, err

}

// Verify is a free data retrieval call binding the contract method 0x75e36616.
//
// Solidity: function verify(bytes32 docHash) view returns(bool found, (string,uint8,address,uint64,uint8,bytes32) cred)
func (_CredentialRegistry *CredentialRegistrySession) Verify(docHash [32]byte) (struct {
	Found bool
	Cred  CredentialRegistryCredential
}, error) {
	return _CredentialRegistry.Contract.Verify(&_CredentialRegistry.CallOpts, docHash)
}

// Verify is a free data retrieval call binding the contract method 0x75e36616.
//
// Solidity: function verify(bytes32 docHash) view returns(bool found, (string,uint8,address,uint64,uint8,bytes32) cred)
func (_CredentialRegistry *CredentialRegistryCallerSession) Verify(docHash [32]byte) (struct {
	Found bool
	Cred  CredentialRegistryCredential
}, error) {
	return _CredentialRegistry.Contract.Verify(&_CredentialRegistry.CallOpts, docHash)
}

// GrantRole is a paid mutator transaction binding the contract method 0x3e840236.
//
// Solidity: function grantRole(address dept, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactor) GrantRole(opts *bind.TransactOpts, dept common.Address, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.contract.Transact(opts, "grantRole", dept, docType)
}

// GrantRole is a paid mutator transaction binding the contract method 0x3e840236.
//
// Solidity: function grantRole(address dept, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistrySession) GrantRole(dept common.Address, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.GrantRole(&_CredentialRegistry.TransactOpts, dept, docType)
}

// GrantRole is a paid mutator transaction binding the contract method 0x3e840236.
//
// Solidity: function grantRole(address dept, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactorSession) GrantRole(dept common.Address, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.GrantRole(&_CredentialRegistry.TransactOpts, dept, docType)
}

// Issue is a paid mutator transaction binding the contract method 0x4c6bfcd7.
//
// Solidity: function issue(bytes32 docHash, string docId, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactor) Issue(opts *bind.TransactOpts, docHash [32]byte, docId string, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.contract.Transact(opts, "issue", docHash, docId, docType)
}

// Issue is a paid mutator transaction binding the contract method 0x4c6bfcd7.
//
// Solidity: function issue(bytes32 docHash, string docId, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistrySession) Issue(docHash [32]byte, docId string, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.Issue(&_CredentialRegistry.TransactOpts, docHash, docId, docType)
}

// Issue is a paid mutator transaction binding the contract method 0x4c6bfcd7.
//
// Solidity: function issue(bytes32 docHash, string docId, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactorSession) Issue(docHash [32]byte, docId string, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.Issue(&_CredentialRegistry.TransactOpts, docHash, docId, docType)
}

// Revoke is a paid mutator transaction binding the contract method 0x917758fe.
//
// Solidity: function revoke(bytes32 docHash, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactor) Revoke(opts *bind.TransactOpts, docHash [32]byte, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.contract.Transact(opts, "revoke", docHash, docType)
}

// Revoke is a paid mutator transaction binding the contract method 0x917758fe.
//
// Solidity: function revoke(bytes32 docHash, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistrySession) Revoke(docHash [32]byte, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.Revoke(&_CredentialRegistry.TransactOpts, docHash, docType)
}

// Revoke is a paid mutator transaction binding the contract method 0x917758fe.
//
// Solidity: function revoke(bytes32 docHash, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactorSession) Revoke(docHash [32]byte, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.Revoke(&_CredentialRegistry.TransactOpts, docHash, docType)
}

// Supersede is a paid mutator transaction binding the contract method 0x7a268326.
//
// Solidity: function supersede(bytes32 oldHash, bytes32 newHash, string docId, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactor) Supersede(opts *bind.TransactOpts, oldHash [32]byte, newHash [32]byte, docId string, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.contract.Transact(opts, "supersede", oldHash, newHash, docId, docType)
}

// Supersede is a paid mutator transaction binding the contract method 0x7a268326.
//
// Solidity: function supersede(bytes32 oldHash, bytes32 newHash, string docId, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistrySession) Supersede(oldHash [32]byte, newHash [32]byte, docId string, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.Supersede(&_CredentialRegistry.TransactOpts, oldHash, newHash, docId, docType)
}

// Supersede is a paid mutator transaction binding the contract method 0x7a268326.
//
// Solidity: function supersede(bytes32 oldHash, bytes32 newHash, string docId, uint8 docType) returns()
func (_CredentialRegistry *CredentialRegistryTransactorSession) Supersede(oldHash [32]byte, newHash [32]byte, docId string, docType uint8) (*types.Transaction, error) {
	return _CredentialRegistry.Contract.Supersede(&_CredentialRegistry.TransactOpts, oldHash, newHash, docId, docType)
}

// CredentialRegistryIssuedIterator is returned from FilterIssued and is used to iterate over the raw logs and unpacked data for Issued events raised by the CredentialRegistry contract.
type CredentialRegistryIssuedIterator struct {
	Event *CredentialRegistryIssued // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CredentialRegistryIssuedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CredentialRegistryIssued)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CredentialRegistryIssued)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CredentialRegistryIssuedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CredentialRegistryIssuedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CredentialRegistryIssued represents a Issued event raised by the CredentialRegistry contract.
type CredentialRegistryIssued struct {
	DocHash [32]byte
	DocId   string
	DocType uint8
	Issuer  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterIssued is a free log retrieval operation binding the contract event 0xeb8d678ec522454c0638e118e9ade4e0ddb89929cb23a4b379d529c6c06aaedc.
//
// Solidity: event Issued(bytes32 indexed docHash, string docId, uint8 docType, address indexed issuer)
func (_CredentialRegistry *CredentialRegistryFilterer) FilterIssued(opts *bind.FilterOpts, docHash [][32]byte, issuer []common.Address) (*CredentialRegistryIssuedIterator, error) {

	var docHashRule []interface{}
	for _, docHashItem := range docHash {
		docHashRule = append(docHashRule, docHashItem)
	}

	var issuerRule []interface{}
	for _, issuerItem := range issuer {
		issuerRule = append(issuerRule, issuerItem)
	}

	logs, sub, err := _CredentialRegistry.contract.FilterLogs(opts, "Issued", docHashRule, issuerRule)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistryIssuedIterator{contract: _CredentialRegistry.contract, event: "Issued", logs: logs, sub: sub}, nil
}

// WatchIssued is a free log subscription operation binding the contract event 0xeb8d678ec522454c0638e118e9ade4e0ddb89929cb23a4b379d529c6c06aaedc.
//
// Solidity: event Issued(bytes32 indexed docHash, string docId, uint8 docType, address indexed issuer)
func (_CredentialRegistry *CredentialRegistryFilterer) WatchIssued(opts *bind.WatchOpts, sink chan<- *CredentialRegistryIssued, docHash [][32]byte, issuer []common.Address) (event.Subscription, error) {

	var docHashRule []interface{}
	for _, docHashItem := range docHash {
		docHashRule = append(docHashRule, docHashItem)
	}

	var issuerRule []interface{}
	for _, issuerItem := range issuer {
		issuerRule = append(issuerRule, issuerItem)
	}

	logs, sub, err := _CredentialRegistry.contract.WatchLogs(opts, "Issued", docHashRule, issuerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CredentialRegistryIssued)
				if err := _CredentialRegistry.contract.UnpackLog(event, "Issued", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseIssued is a log parse operation binding the contract event 0xeb8d678ec522454c0638e118e9ade4e0ddb89929cb23a4b379d529c6c06aaedc.
//
// Solidity: event Issued(bytes32 indexed docHash, string docId, uint8 docType, address indexed issuer)
func (_CredentialRegistry *CredentialRegistryFilterer) ParseIssued(log types.Log) (*CredentialRegistryIssued, error) {
	event := new(CredentialRegistryIssued)
	if err := _CredentialRegistry.contract.UnpackLog(event, "Issued", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CredentialRegistryRevokedIterator is returned from FilterRevoked and is used to iterate over the raw logs and unpacked data for Revoked events raised by the CredentialRegistry contract.
type CredentialRegistryRevokedIterator struct {
	Event *CredentialRegistryRevoked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CredentialRegistryRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CredentialRegistryRevoked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CredentialRegistryRevoked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CredentialRegistryRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CredentialRegistryRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CredentialRegistryRevoked represents a Revoked event raised by the CredentialRegistry contract.
type CredentialRegistryRevoked struct {
	DocHash [32]byte
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRevoked is a free log retrieval operation binding the contract event 0xe5af7daed5ab2a2dc5f98d53619f05089c0c14d11a6621f6b906a2366c9a7ab3.
//
// Solidity: event Revoked(bytes32 indexed docHash)
func (_CredentialRegistry *CredentialRegistryFilterer) FilterRevoked(opts *bind.FilterOpts, docHash [][32]byte) (*CredentialRegistryRevokedIterator, error) {

	var docHashRule []interface{}
	for _, docHashItem := range docHash {
		docHashRule = append(docHashRule, docHashItem)
	}

	logs, sub, err := _CredentialRegistry.contract.FilterLogs(opts, "Revoked", docHashRule)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistryRevokedIterator{contract: _CredentialRegistry.contract, event: "Revoked", logs: logs, sub: sub}, nil
}

// WatchRevoked is a free log subscription operation binding the contract event 0xe5af7daed5ab2a2dc5f98d53619f05089c0c14d11a6621f6b906a2366c9a7ab3.
//
// Solidity: event Revoked(bytes32 indexed docHash)
func (_CredentialRegistry *CredentialRegistryFilterer) WatchRevoked(opts *bind.WatchOpts, sink chan<- *CredentialRegistryRevoked, docHash [][32]byte) (event.Subscription, error) {

	var docHashRule []interface{}
	for _, docHashItem := range docHash {
		docHashRule = append(docHashRule, docHashItem)
	}

	logs, sub, err := _CredentialRegistry.contract.WatchLogs(opts, "Revoked", docHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CredentialRegistryRevoked)
				if err := _CredentialRegistry.contract.UnpackLog(event, "Revoked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRevoked is a log parse operation binding the contract event 0xe5af7daed5ab2a2dc5f98d53619f05089c0c14d11a6621f6b906a2366c9a7ab3.
//
// Solidity: event Revoked(bytes32 indexed docHash)
func (_CredentialRegistry *CredentialRegistryFilterer) ParseRevoked(log types.Log) (*CredentialRegistryRevoked, error) {
	event := new(CredentialRegistryRevoked)
	if err := _CredentialRegistry.contract.UnpackLog(event, "Revoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CredentialRegistryRoleGrantedIterator is returned from FilterRoleGranted and is used to iterate over the raw logs and unpacked data for RoleGranted events raised by the CredentialRegistry contract.
type CredentialRegistryRoleGrantedIterator struct {
	Event *CredentialRegistryRoleGranted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CredentialRegistryRoleGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CredentialRegistryRoleGranted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CredentialRegistryRoleGranted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CredentialRegistryRoleGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CredentialRegistryRoleGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CredentialRegistryRoleGranted represents a RoleGranted event raised by the CredentialRegistry contract.
type CredentialRegistryRoleGranted struct {
	Dept    common.Address
	DocType uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleGranted is a free log retrieval operation binding the contract event 0xaa259565575c834bc07e74dca784b4071676133ac78513b431afb6ee7edae121.
//
// Solidity: event RoleGranted(address indexed dept, uint8 docType)
func (_CredentialRegistry *CredentialRegistryFilterer) FilterRoleGranted(opts *bind.FilterOpts, dept []common.Address) (*CredentialRegistryRoleGrantedIterator, error) {

	var deptRule []interface{}
	for _, deptItem := range dept {
		deptRule = append(deptRule, deptItem)
	}

	logs, sub, err := _CredentialRegistry.contract.FilterLogs(opts, "RoleGranted", deptRule)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistryRoleGrantedIterator{contract: _CredentialRegistry.contract, event: "RoleGranted", logs: logs, sub: sub}, nil
}

// WatchRoleGranted is a free log subscription operation binding the contract event 0xaa259565575c834bc07e74dca784b4071676133ac78513b431afb6ee7edae121.
//
// Solidity: event RoleGranted(address indexed dept, uint8 docType)
func (_CredentialRegistry *CredentialRegistryFilterer) WatchRoleGranted(opts *bind.WatchOpts, sink chan<- *CredentialRegistryRoleGranted, dept []common.Address) (event.Subscription, error) {

	var deptRule []interface{}
	for _, deptItem := range dept {
		deptRule = append(deptRule, deptItem)
	}

	logs, sub, err := _CredentialRegistry.contract.WatchLogs(opts, "RoleGranted", deptRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CredentialRegistryRoleGranted)
				if err := _CredentialRegistry.contract.UnpackLog(event, "RoleGranted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleGranted is a log parse operation binding the contract event 0xaa259565575c834bc07e74dca784b4071676133ac78513b431afb6ee7edae121.
//
// Solidity: event RoleGranted(address indexed dept, uint8 docType)
func (_CredentialRegistry *CredentialRegistryFilterer) ParseRoleGranted(log types.Log) (*CredentialRegistryRoleGranted, error) {
	event := new(CredentialRegistryRoleGranted)
	if err := _CredentialRegistry.contract.UnpackLog(event, "RoleGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CredentialRegistrySupersededIterator is returned from FilterSuperseded and is used to iterate over the raw logs and unpacked data for Superseded events raised by the CredentialRegistry contract.
type CredentialRegistrySupersededIterator struct {
	Event *CredentialRegistrySuperseded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CredentialRegistrySupersededIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CredentialRegistrySuperseded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CredentialRegistrySuperseded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CredentialRegistrySupersededIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CredentialRegistrySupersededIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CredentialRegistrySuperseded represents a Superseded event raised by the CredentialRegistry contract.
type CredentialRegistrySuperseded struct {
	OldHash [32]byte
	NewHash [32]byte
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterSuperseded is a free log retrieval operation binding the contract event 0x4ceca62373493882221cf0c741c782342cd834cd1a8e5480df855f94da186b24.
//
// Solidity: event Superseded(bytes32 indexed oldHash, bytes32 indexed newHash)
func (_CredentialRegistry *CredentialRegistryFilterer) FilterSuperseded(opts *bind.FilterOpts, oldHash [][32]byte, newHash [][32]byte) (*CredentialRegistrySupersededIterator, error) {

	var oldHashRule []interface{}
	for _, oldHashItem := range oldHash {
		oldHashRule = append(oldHashRule, oldHashItem)
	}
	var newHashRule []interface{}
	for _, newHashItem := range newHash {
		newHashRule = append(newHashRule, newHashItem)
	}

	logs, sub, err := _CredentialRegistry.contract.FilterLogs(opts, "Superseded", oldHashRule, newHashRule)
	if err != nil {
		return nil, err
	}
	return &CredentialRegistrySupersededIterator{contract: _CredentialRegistry.contract, event: "Superseded", logs: logs, sub: sub}, nil
}

// WatchSuperseded is a free log subscription operation binding the contract event 0x4ceca62373493882221cf0c741c782342cd834cd1a8e5480df855f94da186b24.
//
// Solidity: event Superseded(bytes32 indexed oldHash, bytes32 indexed newHash)
func (_CredentialRegistry *CredentialRegistryFilterer) WatchSuperseded(opts *bind.WatchOpts, sink chan<- *CredentialRegistrySuperseded, oldHash [][32]byte, newHash [][32]byte) (event.Subscription, error) {

	var oldHashRule []interface{}
	for _, oldHashItem := range oldHash {
		oldHashRule = append(oldHashRule, oldHashItem)
	}
	var newHashRule []interface{}
	for _, newHashItem := range newHash {
		newHashRule = append(newHashRule, newHashItem)
	}

	logs, sub, err := _CredentialRegistry.contract.WatchLogs(opts, "Superseded", oldHashRule, newHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CredentialRegistrySuperseded)
				if err := _CredentialRegistry.contract.UnpackLog(event, "Superseded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSuperseded is a log parse operation binding the contract event 0x4ceca62373493882221cf0c741c782342cd834cd1a8e5480df855f94da186b24.
//
// Solidity: event Superseded(bytes32 indexed oldHash, bytes32 indexed newHash)
func (_CredentialRegistry *CredentialRegistryFilterer) ParseSuperseded(log types.Log) (*CredentialRegistrySuperseded, error) {
	event := new(CredentialRegistrySuperseded)
	if err := _CredentialRegistry.contract.UnpackLog(event, "Superseded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
