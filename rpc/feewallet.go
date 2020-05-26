package rpc

import (
	"context"
	"encoding/hex"
	"fmt"

	wallettypes "decred.org/dcrwallet/rpc/jsonrpc/types"
	"github.com/decred/dcrd/blockchain/stake/v3"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v3"
	dcrdtypes "github.com/decred/dcrd/rpc/jsonrpc/types/v2"
	"github.com/decred/dcrd/wire"
)

const (
	requiredFeeWalletVersion = "8.1.0"
)

// FeeWalletRPC provides methods for calling dcrwallet JSON-RPCs without exposing the details
// of JSON encoding.
type FeeWalletRPC struct {
	Caller
	ctx context.Context
}

// FeeWalletClient creates a new WalletRPC client instance from a caller.
func FeeWalletClient(ctx context.Context, c Caller) (*FeeWalletRPC, error) {

	// Verify dcrwallet is at the required api version.
	var verMap map[string]dcrdtypes.VersionResult
	err := c.Call(ctx, "version", &verMap)
	if err != nil {
		return nil, fmt.Errorf("version check failed: %v", err)
	}
	walletVersion, exists := verMap["dcrwalletjsonrpcapi"]
	if !exists {
		return nil, fmt.Errorf("version response missing 'dcrwalletjsonrpcapi'")
	}
	if walletVersion.VersionString != requiredFeeWalletVersion {
		return nil, fmt.Errorf("wrong dcrwallet RPC version: expected %s, got %s",
			walletVersion.VersionString, requiredFeeWalletVersion)
	}

	// Verify dcrwallet is connected to dcrd (not SPV).
	var walletInfo wallettypes.WalletInfoResult
	err = c.Call(ctx, "walletinfo", &walletInfo)
	if err != nil {
		return nil, fmt.Errorf("walletinfo check failed: %v", err)
	}
	if !walletInfo.DaemonConnected {
		return nil, fmt.Errorf("wallet is not connected to dcrd")
	}

	// TODO: Ensure correct network.

	return &FeeWalletRPC{c, ctx}, nil
}

func (c *FeeWalletRPC) GetBlockHeader(blockHash string) (*dcrdtypes.GetBlockHeaderVerboseResult, error) {
	verbose := true
	var blockHeader dcrdtypes.GetBlockHeaderVerboseResult
	err := c.Call(c.ctx, "getblockheader", &blockHeader, blockHash, verbose)
	if err != nil {
		return nil, err
	}
	return &blockHeader, nil
}

func (c *FeeWalletRPC) GetRawTransaction(txHash string) (*dcrdtypes.TxRawResult, error) {
	verbose := 1
	var resp dcrdtypes.TxRawResult
	err := c.Call(c.ctx, "getrawtransaction", &resp, txHash, verbose)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *FeeWalletRPC) SendRawTransaction(txHex string) (string, error) {
	allowHighFees := false
	var txHash string
	err := c.Call(c.ctx, "sendrawtransaction", &txHash, txHex, allowHighFees)
	if err != nil {
		return "", err
	}
	return txHash, nil
}

func (c *FeeWalletRPC) GetWalletFee() (dcrutil.Amount, error) {
	var amount dcrutil.Amount
	var feeF float64
	err := c.Call(c.ctx, "getwalletfee", &feeF)
	if err != nil {
		return amount, err
	}

	amount, err = dcrutil.NewAmount(feeF)
	if err != nil {
		return amount, err
	}

	return amount, nil
}

func (c *FeeWalletRPC) GetTicketCommitmentAddress(ticketHash string, netParams *chaincfg.Params) (string, error) {
	resp, err := c.GetRawTransaction(ticketHash)
	if err != nil {
		return "", err
	}

	msgHex, err := hex.DecodeString(resp.Hex)
	if err != nil {
		return "", err
	}
	msgTx := wire.NewMsgTx()
	if err = msgTx.FromBytes(msgHex); err != nil {
		return "", err
	}
	addr, err := stake.AddrFromSStxPkScrCommitment(msgTx.TxOut[1].PkScript, netParams)
	if err != nil {
		return "", err
	}

	return addr.Address(), nil
}