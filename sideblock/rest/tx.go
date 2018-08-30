package rest

import (
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/gorilla/mux"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"net/http"
	"encoding/json"
	sdk "github.com/cosmos/cosmos-sdk/types"

	authctx "github.com/cosmos/cosmos-sdk/x/auth/client/context"
	"github.com/basecoin/sideblock"
	"fmt"
	"io/ioutil"
)

func registerTxRoutes(cliCtx context.CLIContext, r *mux.Router, cdc *wire.Codec, kb keys.Keybase) {
	r.HandleFunc(
		"/sideblock/submitBlock",
		sideblockRequestHandlerFn(cdc, kb, cliCtx),
	).Methods("POST")

}
type SideBlockBody struct {
	LocalAccountName string `json:"name"`
	Password         string `json:"password"`
	ChainID          string `json:"chainid"`
	AccountNumber    int64  `json:"accountnumber"`
	Sequence         int64  `json:"sequence"`
	Gas              int64  `json:"gas"`
	VariableAddr    string `json:"validatoraddr"`
	BlockHash 		string `json:"blockhash"`
	TxRoot 			string `json:"txroot"`
	ReceiptRoot 	string `json:"receiptroot"`
}
func sideblockRequestHandlerFn(cdc *wire.Codec, kb keys.Keybase, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var m SideBlockBody

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		err = json.Unmarshal(body, &m)
		if err != nil {
			fmt.Printf("we have error")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}


		//info, err := kb.Get(m.LocalAccountName)
		//if err != nil {
		//	w.WriteHeader(http.StatusUnauthorized)
		//	w.Write([]byte(err.Error()))
		//	return
		//}
		//fmt.Printf("info is %v",info)

		txCtx := authctx.TxContext{
			Codec:         cdc,
			ChainID:       m.ChainID,
			AccountNumber: m.AccountNumber,
			Sequence:      m.Sequence,
			Gas:           m.Gas,
		}

		//variableAddress, err := sdk.AccAddressFromBech32(m.VariableAddr)
		//if err != nil {
		//	w.WriteHeader(http.StatusInternalServerError)
		//	w.Write([]byte(fmt.Sprintf("Couldn't decode address. Error: %s", err.Error())))
		//	return
		//}
		//fmt.Println(variableAddress)
		//fmt.Println(txCtx)
		msg := sideBlock.NewMsgSideBlock(sdk.AccAddress(m.VariableAddr),string(m.BlockHash),string(m.TxRoot),string(m.ReceiptRoot))

		txBytes, err := txCtx.BuildAndSign(m.LocalAccountName, m.Password, []sdk.Msg{msg})
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}

		res, err := cliCtx.BroadcastTx(txBytes)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		output, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.Write(output)

	}
}
