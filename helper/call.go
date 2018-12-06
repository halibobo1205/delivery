package helper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/maticnetwork/heimdall/types"
)

//
//func GetValidators() (validators []abci.ValidatorUpdate) {
//	stakeManagerInstance, err := GetStakeManagerInstance()
//	if err != nil {
//		Logger.Error("Error creating validatorSetInstance", "error", err)
//	}
//
//	powers, _, err := stakeManagerInstance.GetValidatorSet(nil)
//	if err != nil {
//		Logger.Error("Error getting validator set", "error", err)
//	}
//
//	for index := range powers {
//		pubkey, err := stakeManagerInstance.GetPubkey(nil, big.NewInt(int64(index)))
//		if err != nil {
//			Logger.Error("Error getting pubkey for index", "error", err)
//		}
//
//		var pubkeyBytes secp256k1.PubKeySecp256k1
//		_pubkey, _ := hex.DecodeString(pubkey)
//		copy(pubkeyBytes[:], _pubkey)
//		// todo use new valiator update here
//		validator := abci.ValidatorUpdate{
//			Power:  powers[index].Int64(),
//			PubKey: tmtypes.TM2PB.PubKey(pubkeyBytes),
//		}
//
//		validators = append(validators, validator)
//	}
//
//	return validators
//}

// GetHeaderInfo get header info from header id
func GetHeaderInfo(headerID uint64) (root common.Hash, start uint64, end uint64, err error) {
	// get rootchain instance
	rootChainInstance, err := GetRootChainInstance()
	if err != nil {
		Logger.Error("Error creating rootchain instance while fetching headerBlock", "error", err, "headerBlockIndex", headerID)
		return common.Hash{}, 0, 0, err
	}

	// get header from rootchain
	headerIDInt := big.NewInt(0)
	headerIDInt.SetUint64(headerID)
	headerBlock, err := rootChainInstance.HeaderBlock(nil, headerIDInt)
	if err != nil {
		Logger.Error("Unable to fetch header block from rootchain", "headerBlockIndex", headerID)
	}

	return headerBlock.Root, headerBlock.Start.Uint64(), headerBlock.End.Uint64(), nil
}

// GetValidatorInfo get validator info
func GetValidatorInfo(addr common.Address) (validator types.Validator, err error) {
	// get stakemanager intance
	stakeManagerInstance, err := GetStakeManagerInstance()
	if err != nil {
		Logger.Error("Error creating stakeManagerInstance while getting validator info", "error", err, "validatorAddrress", addr)
		return
	}

	amount, startEpoch, endEpoch, signer, err := stakeManagerInstance.GetStakerDetails(nil, addr)
	if err != nil {
		Logger.Error("Error fetching validator information from stakemanager", "Error", err, "ValidatorAddress", addr)
		return
	}
	validator = types.Validator{
		Address:    addr,
		Power:      amount.Uint64(),
		StartEpoch: startEpoch.Uint64(),
		EndEpoch:   endEpoch.Uint64(),
		Signer:     signer,
	}

	return validator, nil
}

// CurrentChildBlock fetch current child block
func CurrentChildBlock() (uint64, error) {
	rootChainInstance, err := GetRootChainInstance()
	if err != nil {
		Logger.Error("Error creating rootchain instance while fetching current child block", "error", err)
		return 0, err
	}

	currentChildBlock, err := rootChainInstance.CurrentChildBlock(nil)
	if err != nil {
		Logger.Error("Could not fetch current child block from rootchain contract", "Error", err)
		return 0, err
	}

	return currentChildBlock.Uint64(), nil
}
