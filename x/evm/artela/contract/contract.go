package contract

import (
	"strings"

	"cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core"

	"github.com/artela-network/artela-evm/vm"
	"github.com/artela-network/artela-rollkit/common"
	"github.com/artela-network/artela-rollkit/common/aspect"
	"github.com/artela-network/artela-rollkit/x/evm/states"
	evmtypes "github.com/artela-network/artela-rollkit/x/evm/types"
)

type AspectNativeContract struct {
	evmState *states.StateDB
	evm      *vm.EVM

	logger   log.Logger
	handlers map[string]Handler

	storeService       store.KVStoreService
	aspectStoreService store.KVStoreService
}

func NewAspectNativeContract(
	storeService store.KVStoreService,
	aspectStoreService store.KVStoreService,
	evm *vm.EVM,
	evmState *states.StateDB,
	logger log.Logger,
) *AspectNativeContract {
	return &AspectNativeContract{
		storeService:       storeService,
		aspectStoreService: aspectStoreService,
		evm:                evm,
		evmState:           evmState,
		logger:             logger,
		handlers:           make(map[string]Handler),
	}
}

func (c *AspectNativeContract) Init() {
	c.register(DeployHandler{})
	c.register(UpgradeHandler{})
	c.register(BindHandler{})
	c.register(UnbindHandler{})
	c.register(ChangeVersionHandler{})
	c.register(GetVersionHandler{})
	c.register(GetBindingHandler{})
	c.register(GetBoundAddressHandler{})
	c.register(OperationHandler{})
}

func (c *AspectNativeContract) register(handler Handler) {
	c.handlers[handler.Method()] = handler
}

func (c *AspectNativeContract) ApplyMessage(ctx sdk.Context, msg *core.Message, gas uint64, commit bool) (ret []byte, remainingGas uint64, err error) {
	return c.applyMsg(ctx, msg, gas, commit)
}

func (c *AspectNativeContract) applyMsg(ctx sdk.Context, msg *core.Message, gas uint64, commit bool) (ret []byte, remainingGas uint64, err error) {
	method, parameters, err := aspect.ParseMethod(msg.Data)
	if err != nil {
		return nil, 0, err
	}

	handler, ok := c.handlers[strings.ToLower(method.Name)]
	if !ok {
		return nil, 0, errorsmod.Wrapf(evmtypes.ErrCallContract, "method %s not found", method.Name)
	}

	handlerCtx := &HandlerContext{
		ctx,
		msg.From,
		parameters,
		commit,
		common.WrapLogger(c.logger.With("module", "aspect-system-contract")),
		c.evmState,
		c.evm,
		method,
		c.storeService,
		c.aspectStoreService,
		msg.Data,
		msg.Nonce,
		msg.GasLimit,
		msg.GasPrice,
		msg.GasTipCap,
		msg.GasFeeCap,
	}

	return handler.Handle(handlerCtx, gas)
}
