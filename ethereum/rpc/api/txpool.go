package api

import (
	"fmt"
	"strconv"

	evmtypes "github.com/artela-network/artela-rollkit/x/evm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	rpctypes "github.com/artela-network/artela-rollkit/ethereum/rpc/types"
)

// TxPoolAPI offers and API for the transaction pool. It only operates on data that is non-confidential.
type TxPoolAPI struct {
	b      rpctypes.TxPoolBackend
	logger log.Logger
}

// NewTxPoolAPI creates a new tx pool service that gives information about the transaction pool.
func NewTxPoolAPI(b rpctypes.TxPoolBackend, logger log.Logger) *TxPoolAPI {
	return &TxPoolAPI{b, logger}
}

// Content returns the transactions contained within the transaction pool.
func (s *TxPoolAPI) Content() map[string]map[string]map[string]*rpctypes.RPCTransaction {
	content := map[string]map[string]map[string]*rpctypes.RPCTransaction{
		"pending": make(map[string]map[string]*rpctypes.RPCTransaction),
		"queued":  s.getPendingContent(common.Address{}),
	}

	return content
}

// ContentFrom returns the transactions contained within the transaction pool.
func (s *TxPoolAPI) ContentFrom(address common.Address) map[string]map[string]*rpctypes.RPCTransaction {
	return s.getPendingContent(address)
}

// Status returns the number of pending and queued transaction in the pool.
func (s *TxPoolAPI) Status() map[string]hexutil.Uint {
	pending, err := s.b.PendingTransactionsCount()
	if err != nil {
		s.logger.Debug("get pending transaction count failed", "error", err.Error())
	}
	return map[string]hexutil.Uint{
		"pending": hexutil.Uint(pending),
		"queued":  hexutil.Uint(0),
	}
}

// Inspect retrieves the content of the transaction pool and flattens it into an
// easily inspectable list.
func (s *TxPoolAPI) Inspect() map[string]map[string]map[string]string {
	content := map[string]map[string]map[string]string{
		"pending": make(map[string]map[string]string),
		"queued":  make(map[string]map[string]string),
	}
	pending := s.getPendingContent(common.Address{})

	// Define a formatter to flatten a transaction into a string
	var format = func(tx *rpctypes.RPCTransaction) string {
		if to := tx.To; to != nil {
			return fmt.Sprintf("%s: %v wei + %v gas × %v wei", tx.To.Hex(), tx.Value, tx.Gas, tx.GasPrice)
		}
		return fmt.Sprintf("contract creation: %v wei + %v gas × %v wei", tx.Value, tx.Gas, tx.GasPrice)
	}
	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce)] = format(tx)
		}
		content["pending"][account] = dump
	}
	return content
}

func (s *TxPoolAPI) getPendingContent(addr common.Address) map[string]map[string]*rpctypes.RPCTransaction {
	pendingContent := make(map[string]map[string]*rpctypes.RPCTransaction)
	pendingTxs, err := s.b.PendingTransactions()
	if err != nil {
		s.logger.Debug("txpool_context, get pending transactions failed", "err", err.Error())
		return pendingContent
	}

	cfg := s.b.ChainConfig()
	if cfg == nil {
		s.logger.Debug("txpool_context, failed to get chain config")
		return pendingContent
	}
	for _, tx := range pendingTxs {
		for _, msg := range (*tx).GetMsgs() {
			if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
				sender, err := s.b.GetSender(ethMsg, cfg.ChainID)
				if err != nil {
					s.logger.Debug("txpool_context, get pending transaction sender", "err", err.Error())
					continue
				}

				if (addr != common.Address{} && addr != sender) {
					continue
				}

				txData, err := evmtypes.UnpackTxData(ethMsg.Data)
				if err != nil {
					s.logger.Debug("txpool_context, unpack pending transaction failed", "err", err.Error())
					continue
				}

				rpctx := rpctypes.NewTransactionFromMsg(ethMsg, common.Hash{}, uint64(0), uint64(0), nil, cfg)
				if pendingContent[sender.String()] == nil {
					pendingContent[sender.String()] = make(map[string]*rpctypes.RPCTransaction)
				}
				pendingContent[sender.String()][strconv.FormatUint(txData.GetNonce(), 10)] = rpctx
			}
		}
	}
	return pendingContent
}
