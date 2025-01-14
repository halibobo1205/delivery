package checkpoint

import (
	"bytes"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ethCommon "github.com/maticnetwork/bor/common"
	abci "github.com/tendermint/tendermint/abci/types"
	tmTypes "github.com/tendermint/tendermint/types"

	"github.com/maticnetwork/heimdall/checkpoint/types"
	"github.com/maticnetwork/heimdall/common"
	"github.com/maticnetwork/heimdall/helper"
	hmTypes "github.com/maticnetwork/heimdall/types"
)

// NewSideTxHandler returns a side handler for "bank" type messages.
func NewSideTxHandler(k Keeper, contractCaller helper.IContractCaller) hmTypes.SideTxHandler {
	return func(ctx sdk.Context, msg sdk.Msg) abci.ResponseDeliverSideTx {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case types.MsgCheckpoint:
			return SideHandleMsgCheckpoint(ctx, k, msg, contractCaller)
		case types.MsgCheckpointAck:
			return SideHandleMsgCheckpointAck(ctx, k, msg, contractCaller)
		case types.MsgCheckpointSync:
			return SideHandleMsgCheckpointSync(ctx, k, msg, contractCaller)
		case types.MsgCheckpointSyncAck:
			return SideHandleMsgCheckpointSyncAck(ctx, k, msg, contractCaller)
		default:
			return abci.ResponseDeliverSideTx{
				Code: uint32(sdk.CodeUnknownRequest),
			}
		}
	}
}

// SideHandleMsgCheckpoint handles MsgCheckpoint message for external call
func SideHandleMsgCheckpoint(ctx sdk.Context, k Keeper, msg types.MsgCheckpoint, contractCaller helper.IContractCaller) (result abci.ResponseDeliverSideTx) {
	// get params
	params := k.GetParams(ctx)

	// logger
	logger := k.Logger(ctx)

	// validate checkpoint
	validCheckpoint, err := types.ValidateCheckpoint(msg.StartBlock, msg.EndBlock, msg.RootHash, params.MaxCheckpointLength, contractCaller)
	if err != nil {
		logger.Error("Error validating checkpoint",
			"error", err,
			"startBlock", msg.StartBlock,
			"endBlock", msg.EndBlock,
		)
	} else if validCheckpoint {
		// vote `yes` if checkpoint is valid
		result.Result = abci.SideTxResultType_Yes
		return
	}

	logger.Error(
		"RootHash is not valid",
		"startBlock", msg.StartBlock,
		"endBlock", msg.EndBlock,
		"rootHash", msg.RootHash,
	)

	return common.ErrorSideTx(k.Codespace(), common.CodeInvalidBlockInput)
}

// SideHandleMsgCheckpointAck handles MsgCheckpointAck message for external call
func SideHandleMsgCheckpointAck(ctx sdk.Context, k Keeper, msg types.MsgCheckpointAck, contractCaller helper.IContractCaller) (result abci.ResponseDeliverSideTx) {
	if msg.RootChainType == hmTypes.RootChainTypeTron {
		return SideHandleMsgTronCheckpointAck(ctx, k, msg, contractCaller)
	}

	logger := k.Logger(ctx)
	logger.Debug("✅ Validating External call for checkpoint ack msg",
		"root", msg.RootChainType,
		"start", msg.StartBlock,
		"end", msg.EndBlock,
		"number", msg.Number,
	)

	params := k.GetParams(ctx)
	chainParams := k.ck.GetParams(ctx).ChainParams

	//
	// Validate data from root chain
	//
	var rootChainAddress ethCommon.Address
	switch msg.RootChainType {
	case hmTypes.RootChainTypeEth:
		rootChainAddress = chainParams.RootChainAddress.EthAddress()
	case hmTypes.RootChainTypeBsc:
		bscChain, err := k.ck.GetChainParams(ctx, msg.RootChainType)
		if err != nil {
			k.Logger(ctx).Error("RootChain type: ", msg.RootChainType, "does not match bsc")
			return common.ErrorSideTx(k.Codespace(), common.CodeWrongRootChainType)
		}
		rootChainAddress = bscChain.RootChainAddress.EthAddress()
	}
	rootChainInstance, err := contractCaller.GetRootChainInstance(rootChainAddress, msg.RootChainType)
	if err != nil {
		logger.Error("Unable to fetch rootchain contract instance", "error", err)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}

	root, start, end, _, proposer, err := contractCaller.GetHeaderInfo(msg.Number, rootChainInstance, params.ChildBlockInterval)
	if err != nil {
		logger.Error("Unable to fetch checkpoint from rootchain", "error", err, "checkpointNumber", msg.Number)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}

	// check if message data matches with contract data
	if msg.StartBlock != start ||
		msg.EndBlock != end ||
		!msg.Proposer.Equals(proposer) ||
		!bytes.Equal(msg.RootHash.Bytes(), root.Bytes()) {

		logger.Error("Invalid message. It doesn't match with contract state", "error", err, "checkpointNumber", msg.Number)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}

	// say `yes`
	result.Result = abci.SideTxResultType_Yes

	return
}

// SideHandleMsgTronCheckpointAck handles MsgCheckpointAck message for external call
func SideHandleMsgTronCheckpointAck(ctx sdk.Context, k Keeper, msg types.MsgCheckpointAck, contractCaller helper.IContractCaller) (result abci.ResponseDeliverSideTx) {
	logger := k.Logger(ctx)

	params := k.GetParams(ctx)
	chainParams := k.ck.GetParams(ctx).ChainParams

	//
	// Validate data from root chain
	//
	if msg.RootChainType == hmTypes.RootChainTypeTron {
		root, start, end, _, proposer, err := contractCaller.GetTronHeaderInfo(msg.Number, chainParams.TronChainAddress, params.ChildBlockInterval)
		if err != nil {
			logger.Error("Unable to fetch checkpoint from tron", "error", err, "checkpointNumber", msg.Number)
			return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
		}

		// check if message data matches with contract data
		if msg.StartBlock != start ||
			msg.EndBlock != end ||
			!msg.Proposer.Equals(proposer) ||
			!bytes.Equal(msg.RootHash.Bytes(), root.Bytes()) {

			logger.Error("Invalid message. It doesn't match with contract state", "error", err, "checkpointNumber", msg.Number)
			return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
		}
		// say `yes`
		result.Result = abci.SideTxResultType_Yes
		return
	}

	return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
}

// SideHandleMsgCheckpointSync handles MsgCheckpointSync message for external call
func SideHandleMsgCheckpointSync(ctx sdk.Context, k Keeper, msg types.MsgCheckpointSync, contractCaller helper.IContractCaller) (result abci.ResponseDeliverSideTx) {
	// logger
	logger := k.Logger(ctx)
	logger.Debug("✅ Validating External call for checkpoint sync msg",
		"root", msg.RootChainType,
		"number", msg.Number,
	)

	params := k.GetParams(ctx)
	chainParams := k.ck.GetParams(ctx).ChainParams

	//
	// Validate data from root chain
	//
	var (
		start, end uint64
		proposer   hmTypes.HeimdallAddress
		err        error
	)

	var rootChainAddress ethCommon.Address
	switch msg.RootChainType {
	case hmTypes.RootChainTypeEth:
		rootChainAddress = chainParams.RootChainAddress.EthAddress()
	case hmTypes.RootChainTypeBsc:
		bscChain, err := k.ck.GetChainParams(ctx, msg.RootChainType)
		if err != nil {
			k.Logger(ctx).Error("RootChain type: ", msg.RootChainType, "does not  match bsc")
			return common.ErrorSideTx(k.Codespace(), common.CodeWrongRootChainType)
		}
		rootChainAddress = bscChain.RootChainAddress.EthAddress()
	}
	rootChainInstance, err := contractCaller.GetRootChainInstance(rootChainAddress, msg.RootChainType)
	if err != nil {
		logger.Error("Unable to fetch rootchain contract instance", "root", msg.RootChainType, "error", err)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}
	_, start, end, _, proposer, err = contractCaller.GetHeaderInfo(msg.Number, rootChainInstance, params.ChildBlockInterval)
	if err != nil {
		logger.Error("Unable to fetch checkpoint from rootchain",
			"root", msg.RootChainType, "error", err, "checkpointNumber", msg.Number)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}

	// check if message data matches with contract data
	if msg.StartBlock != start ||
		msg.EndBlock != end ||
		!msg.Proposer.Equals(proposer) {
		logger.Error("Invalid checkpoint sync message. It doesn't match with contract state",
			"root", msg.RootChainType, "error", err, "checkpointNumber", msg.Number)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}

	// say `yes`
	result.Result = abci.SideTxResultType_Yes
	return
}

// SideHandleMsgCheckpointSyncAck handles MsgCheckpointAck message for external call
func SideHandleMsgCheckpointSyncAck(ctx sdk.Context, k Keeper, msg types.MsgCheckpointSyncAck, contractCaller helper.IContractCaller) (result abci.ResponseDeliverSideTx) {
	// logger
	logger := k.Logger(ctx)

	logger.Debug("✅ Validating External call for checkpoint sync ack msg",
		"root", msg.RootChainType,
		"number", msg.Number,
	)

	chainParams := k.ck.GetParams(ctx).ChainParams

	//
	// Validate data from root chain
	//
	currentNumber, err := contractCaller.GetSyncedCheckpointId(chainParams.TronStakingManagerAddress, msg.RootChainType)
	if err != nil {
		logger.Error("Unable to fetch checkpoint from rootchain", "error", err, "checkpointNumber", msg.Number)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}
	if msg.Number > currentNumber {
		logger.Error("Invalid message. It doesn't match with contract state", "error", err, "checkpointNumber", msg.Number)
		return common.ErrorSideTx(k.Codespace(), common.CodeInvalidACK)
	}

	// say `yes`
	result.Result = abci.SideTxResultType_Yes

	return
}

//
// Tx handler
//

// NewPostTxHandler returns a side handler for "bank" type messages.
func NewPostTxHandler(k Keeper, contractCaller helper.IContractCaller) hmTypes.PostTxHandler {
	return func(ctx sdk.Context, msg sdk.Msg, sideTxResult abci.SideTxResultType) sdk.Result {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case types.MsgCheckpoint:
			return PostHandleMsgCheckpoint(ctx, k, msg, sideTxResult)
		case types.MsgCheckpointAck:
			return PostHandleMsgCheckpointAck(ctx, k, msg, sideTxResult)
		case types.MsgCheckpointSync:
			return PostHandleMsgCheckpointSync(ctx, k, msg, sideTxResult)
		case types.MsgCheckpointSyncAck:
			return PostHandleMsgCheckpointSyncAck(ctx, k, msg, sideTxResult)
		default:
			return sdk.ErrUnknownRequest("Unrecognized checkpoint Msg type").Result()
		}
	}
}

// PostHandleMsgCheckpoint handles msg checkpoint
func PostHandleMsgCheckpoint(ctx sdk.Context, k Keeper, msg types.MsgCheckpoint, sideTxResult abci.SideTxResultType) sdk.Result {
	logger := k.Logger(ctx)

	// Skip handler if checkpoint is not approved
	if sideTxResult != abci.SideTxResultType_Yes {
		logger.Debug("Skipping new checkpoint since side-tx didn't get yes votes", "startBlock", msg.StartBlock, "endBlock", msg.EndBlock, "rootHash", msg.RootHash)
		return common.ErrBadBlockDetails(k.Codespace()).Result()
	}

	//
	// Validate last checkpoint
	//
	lastCheckpoint, err := k.GetLastCheckpoint(ctx, msg.RootChainType)

	// fetch last checkpoint from store
	if err == nil {
		// make sure new checkpoint is after tip
		if lastCheckpoint.EndBlock > msg.StartBlock {
			logger.Error("Checkpoint already exists",
				"currentTip", lastCheckpoint.EndBlock,
				"startBlock", msg.StartBlock,
			)
			return common.ErrOldCheckpoint(k.Codespace()).Result()
		}

		// check if new checkpoint's start block start from current tip
		if lastCheckpoint.EndBlock+1 != msg.StartBlock {
			logger.Error("Checkpoint not in countinuity",
				"currentTip", lastCheckpoint.EndBlock,
				"startBlock", msg.StartBlock,
				"root", msg.RootChainType)
			return common.ErrDisCountinuousCheckpoint(k.Codespace()).Result()
		}
	} else if err.Error() == common.ErrNoCheckpointFound(k.Codespace()).Error() {
		activation := k.ck.GetChainActivationHeight(ctx, msg.RootChainType)
		if activation != msg.StartBlock {
			logger.Error("First checkpoint to start from block active height",
				"activation", activation, "start", msg.StartBlock, "root", msg.RootChainType)
			return common.ErrBadBlockDetails(k.Codespace()).Result()
		}
	}

	//
	// Save checkpoint to buffer store
	//
	checkpointBuffer, err := k.GetCheckpointFromBuffer(ctx, msg.RootChainType)
	if err == nil && checkpointBuffer != nil {
		logger.Debug("Checkpoint already exists in buffer")

		// get checkpoint buffer time from params
		params := k.GetParams(ctx)
		expiryTime := checkpointBuffer.TimeStamp + uint64(params.CheckpointBufferTime.Seconds())

		// return with error (ack is required)
		return common.ErrNoACK(k.Codespace(), expiryTime).Result()
	}

	timeStamp := uint64(ctx.BlockTime().Unix())

	// Add checkpoint to buffer with root hash and account hash
	k.SetCheckpointBuffer(ctx, hmTypes.Checkpoint{
		StartBlock: msg.StartBlock,
		EndBlock:   msg.EndBlock,
		RootHash:   msg.RootHash,
		Proposer:   msg.Proposer,
		BorChainID: msg.BorChainID,
		TimeStamp:  timeStamp,
	}, msg.RootChainType)

	logger.Debug("New checkpoint into buffer stored",
		"startBlock", msg.StartBlock,
		"endBlock", msg.EndBlock,
		"rootHash", msg.RootHash,
		"rootChain", msg.RootChainType,
	)

	// TX bytes
	txBytes := ctx.TxBytes()
	hash := tmTypes.Tx(txBytes).Hash()

	// Emit event for checkpoints
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeCheckpoint,
			sdk.NewAttribute(sdk.AttributeKeyAction, msg.Type()),                                  // action
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),                // module name
			sdk.NewAttribute(hmTypes.AttributeKeyTxHash, hmTypes.BytesToHeimdallHash(hash).Hex()), // tx hash
			sdk.NewAttribute(hmTypes.AttributeKeySideTxResult, sideTxResult.String()),             // result
			sdk.NewAttribute(types.AttributeKeyProposer, msg.Proposer.String()),
			sdk.NewAttribute(types.AttributeKeyStartBlock, strconv.FormatUint(msg.StartBlock, 10)),
			sdk.NewAttribute(types.AttributeKeyEndBlock, strconv.FormatUint(msg.EndBlock, 10)),
			sdk.NewAttribute(types.AttributeKeyRootHash, msg.RootHash.String()),
			sdk.NewAttribute(types.AttributeKeyAccountHash, msg.AccountRootHash.String()),
			sdk.NewAttribute(types.AttributeKeyRootChain, msg.RootChainType),
		),
	})

	return sdk.Result{
		Events: ctx.EventManager().Events(),
	}
}

// PostHandleMsgCheckpointAck handles msg checkpoint ack
func PostHandleMsgCheckpointAck(ctx sdk.Context, k Keeper, msg types.MsgCheckpointAck, sideTxResult abci.SideTxResultType) sdk.Result {
	logger := k.Logger(ctx)

	// Skip handler if checkpoint-ack is not approved
	if sideTxResult != abci.SideTxResultType_Yes {
		logger.Debug("Skipping new checkpoint-ack since side-tx didn't get yes votes",
			"checkpointNumber", msg.Number, "root", msg.RootChainType)
		return common.ErrBadBlockDetails(k.Codespace()).Result()
	}

	// get last checkpoint from buffer
	checkpointObj, err := k.GetCheckpointFromBuffer(ctx, msg.RootChainType)
	if err != nil {
		logger.Error("Unable to get checkpoint buffer", "error", err, "root", msg.RootChainType)
		return common.ErrBadAck(k.Codespace()).Result()
	}

	// invalid start block
	if msg.StartBlock != checkpointObj.StartBlock {
		logger.Error("Invalid start block",
			"startExpected", checkpointObj.StartBlock,
			"startReceived", msg.StartBlock,
			"root", msg.RootChainType)
		return common.ErrBadAck(k.Codespace()).Result()
	}

	// Return err if start and end matches but contract root hash doesn't match
	if msg.StartBlock == checkpointObj.StartBlock && msg.EndBlock == checkpointObj.EndBlock && !msg.RootHash.Equals(checkpointObj.RootHash) {
		logger.Error("Invalid ACK",
			"startExpected", checkpointObj.StartBlock,
			"startReceived", msg.StartBlock,
			"endExpected", checkpointObj.EndBlock,
			"endReceived", msg.StartBlock,
			"rootExpected", checkpointObj.RootHash.String(),
			"rootRecieved", msg.RootHash.String(),
			"rootChain", msg.RootChainType,
		)
		return common.ErrBadAck(k.Codespace()).Result()
	}

	// adjust checkpoint data if latest checkpoint is already submitted
	if checkpointObj.EndBlock > msg.EndBlock {
		logger.Info("Adjusting endBlock to one already submitted on chain",
			"endBlock", checkpointObj.EndBlock, "adjustedEndBlock", msg.EndBlock, "root", msg.RootChainType)
		checkpointObj.EndBlock = msg.EndBlock
		checkpointObj.RootHash = msg.RootHash
		checkpointObj.Proposer = msg.Proposer
	}

	//
	// Update checkpoint state
	//
	err = k.AddCheckpoint(ctx, msg.Number, *checkpointObj, msg.RootChainType)

	// Add checkpoint to store
	if err != nil {
		logger.Error("Error while adding checkpoint into store", "checkpointNumber", msg.Number)
		return sdk.ErrInternal("Failed to add checkpoint into store").Result()
	}
	logger.Debug("Checkpoint added to store", "checkpointNumber", msg.Number, "root", msg.RootChainType)

	// Flush buffer
	k.UpdateACKCount(ctx, msg.RootChainType)
	k.FlushCheckpointBuffer(ctx, msg.RootChainType)

	logger.Debug("Checkpoint buffer flushed after receiving checkpoint ack", "root", msg.RootChainType)

	// Update ack count in staking module
	logger.Info("Valid ack received",
		"CurrentACKCount", k.GetACKCount(ctx, msg.RootChainType)-1,
		"UpdatedACKCount", k.GetACKCount(ctx, msg.RootChainType),
		"root", msg.RootChainType)

	if msg.RootChainType == hmTypes.RootChainTypeStake {
		// Increment accum (selects new proposer)
		k.sk.IncrementAccum(ctx, 1)
	}

	// TX bytes
	txBytes := ctx.TxBytes()
	hash := tmTypes.Tx(txBytes).Hash()

	// Emit event for checkpoints
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeCheckpointAck,
			sdk.NewAttribute(sdk.AttributeKeyAction, msg.Type()),                                  // action
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),                // module name
			sdk.NewAttribute(hmTypes.AttributeKeyTxHash, hmTypes.BytesToHeimdallHash(hash).Hex()), // tx hash
			sdk.NewAttribute(hmTypes.AttributeKeySideTxResult, sideTxResult.String()),             // result
			sdk.NewAttribute(types.AttributeKeyHeaderIndex, strconv.FormatUint(msg.Number, 10)),
			sdk.NewAttribute(types.AttributeKeyRootChain, msg.RootChainType),
		),
	})

	return sdk.Result{
		Events: ctx.EventManager().Events(),
	}
}

// PostHandleMsgCheckpointSync handles msg checkpoint
func PostHandleMsgCheckpointSync(ctx sdk.Context, k Keeper, msg types.MsgCheckpointSync, sideTxResult abci.SideTxResultType) sdk.Result {
	logger := k.Logger(ctx)
	logger.Debug("Post handle checkpoint sync",
		"rootChain", msg.RootChainType,
		"startBlock", msg.StartBlock,
		"endBlock", msg.EndBlock,
		"proposer", msg.Proposer,
		"number", msg.Number,
	)
	// Skip handler if checkpoint is not approved
	if sideTxResult != abci.SideTxResultType_Yes {
		logger.Debug("Skipping new checkpoint sync since side-tx didn't get yes votes",
			"root", msg.RootChainType, "startBlock", msg.StartBlock, "endBlock", msg.EndBlock)
		return common.ErrBadBlockDetails(k.Codespace()).Result()
	}

	//
	// Save checkpoint to buffer store
	//
	checkpointSyncBuffer, err := k.GetCheckpointSyncFromBuffer(ctx, msg.RootChainType)

	if err == nil && checkpointSyncBuffer != nil {
		logger.Debug("Checkpoint sync already exists in buffer")

		// get checkpoint buffer time from params
		params := k.GetParams(ctx)
		expiryTime := checkpointSyncBuffer.TimeStamp + uint64(params.CheckpointBufferTime.Seconds())

		// return with error (ack is required)
		return common.ErrNoACK(k.Codespace(), expiryTime).Result()
	}

	timeStamp := uint64(ctx.BlockTime().Unix())

	k.SetCheckpointSyncBuffer(ctx, hmTypes.Checkpoint{
		StartBlock: msg.StartBlock,
		EndBlock:   msg.EndBlock,
		Proposer:   msg.Proposer,
		TimeStamp:  timeStamp,
	}, msg.RootChainType)

	logger.Debug("New checkpoint sync into buffer stored",
		"rootChain", msg.RootChainType,
		"startBlock", msg.StartBlock,
		"endBlock", msg.EndBlock,
		"proposer", msg.Proposer,
		"number", msg.Number,
	)

	// TX bytes
	txBytes := ctx.TxBytes()
	hash := tmTypes.Tx(txBytes).Hash()

	// Emit event for checkpoints
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeCheckpointSync,
			sdk.NewAttribute(sdk.AttributeKeyAction, msg.Type()),                                  // action
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),                // module name
			sdk.NewAttribute(hmTypes.AttributeKeyTxHash, hmTypes.BytesToHeimdallHash(hash).Hex()), // tx hash
			sdk.NewAttribute(hmTypes.AttributeKeySideTxResult, sideTxResult.String()),             // result
			sdk.NewAttribute(types.AttributeKeyProposer, msg.Proposer.String()),
			sdk.NewAttribute(types.AttributeKeyStartBlock, strconv.FormatUint(msg.StartBlock, 10)),
			sdk.NewAttribute(types.AttributeKeyEndBlock, strconv.FormatUint(msg.EndBlock, 10)),
			sdk.NewAttribute(types.AttributeKeyRootChain, msg.RootChainType),
			sdk.NewAttribute(types.AttributeKeyHeaderIndex, strconv.FormatUint(msg.Number, 10)),
		),
	})

	return sdk.Result{
		Events: ctx.EventManager().Events(),
	}
}

// PostHandleMsgCheckpointSyncAck handles msg checkpoint ack
func PostHandleMsgCheckpointSyncAck(ctx sdk.Context, k Keeper, msg types.MsgCheckpointSyncAck, sideTxResult abci.SideTxResultType) sdk.Result {
	logger := k.Logger(ctx)

	// Skip handler if checkpoint-ack is not approved
	if sideTxResult != abci.SideTxResultType_Yes {
		logger.Debug("Skipping new checkpoint-sync-ack since side-tx didn't get yes votes",
			"checkpointNumber", msg.Number, "root", msg.RootChainType)
		return common.ErrBadBlockDetails(k.Codespace()).Result()
	}

	//
	// Update checkpoint sync state
	//
	k.FlushCheckpointSyncBuffer(ctx, msg.RootChainType)
	logger.Debug("Checkpoint buffer flushed after receiving checkpoint sync ack", "root", msg.RootChainType)

	// TX bytes
	txBytes := ctx.TxBytes()
	hash := tmTypes.Tx(txBytes).Hash()

	// Emit event for checkpoints
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeCheckpointSyncAck,
			sdk.NewAttribute(sdk.AttributeKeyAction, msg.Type()),                                  // action
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),                // module name
			sdk.NewAttribute(hmTypes.AttributeKeyTxHash, hmTypes.BytesToHeimdallHash(hash).Hex()), // tx hash
			sdk.NewAttribute(hmTypes.AttributeKeySideTxResult, sideTxResult.String()),             // result
			sdk.NewAttribute(types.AttributeKeyHeaderIndex, strconv.FormatUint(msg.Number, 10)),
			sdk.NewAttribute(types.AttributeKeyRootChain, msg.RootChainType),
		),
	})

	return sdk.Result{
		Events: ctx.EventManager().Events(),
	}
}
