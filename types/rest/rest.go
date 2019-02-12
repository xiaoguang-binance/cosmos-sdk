// Package rest provides HTTP types and primitives for REST
// requests validation and responses handling.
package rest

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types"
)

// GasEstimateResponse defines a response definition for tx gas estimation.
type GasEstimateResponse struct {
	GasEstimate uint64 `json:"gas_estimate"`
}

// UnjailReq request unjailing
type UnjailReq struct {
	BaseReq BaseReq `json:"base_req"`
}

// BaseReq defines a structure that can be embedded in other request structures
// that all share common "base" fields.
type BaseReq struct {
	From          string         `json:"from"`
	Password      string         `json:"password"`
	Memo          string         `json:"memo"`
	ChainID       string         `json:"chain_id"`
	AccountNumber uint64         `json:"account_number"`
	Sequence      uint64         `json:"sequence"`
	Fees          types.Coins    `json:"fees"`
	GasPrices     types.DecCoins `json:"gas_prices"`
	Gas           string         `json:"gas"`
	GasAdjustment string         `json:"gas_adjustment"`
	GenerateOnly  bool           `json:"generate_only"`
	Simulate      bool           `json:"simulate"`
}

// NewBaseReq creates a new basic request instance and sanitizes its values
func NewBaseReq(
	from, password, memo, chainID string, gas, gasAdjustment string,
	accNumber, seq uint64, fees types.Coins, gasPrices types.DecCoins, genOnly, simulate bool,
) BaseReq {

	return BaseReq{
		From:          strings.TrimSpace(from),
		Password:      password,
		Memo:          strings.TrimSpace(memo),
		ChainID:       strings.TrimSpace(chainID),
		Fees:          fees,
		GasPrices:     gasPrices,
		Gas:           strings.TrimSpace(gas),
		GasAdjustment: strings.TrimSpace(gasAdjustment),
		AccountNumber: accNumber,
		Sequence:      seq,
		GenerateOnly:  genOnly,
		Simulate:      simulate,
	}
}

// Sanitize performs basic sanitization on a BaseReq object.
func (br BaseReq) Sanitize() BaseReq {
	return NewBaseReq(
		br.From, br.Password, br.Memo, br.ChainID, br.Gas, br.GasAdjustment,
		br.AccountNumber, br.Sequence, br.Fees, br.GasPrices, br.GenerateOnly, br.Simulate,
	)
}

// ValidateBasic performs basic validation of a BaseReq. If custom validation
// logic is needed, the implementing request handler should perform those
// checks manually.
func (br BaseReq) ValidateBasic(w http.ResponseWriter) bool {
	if !br.GenerateOnly && !br.Simulate {
		switch {
		case len(br.Password) == 0:
			WriteErrorResponse(w, http.StatusUnauthorized, "password required but not specified")
			return false

		case len(br.ChainID) == 0:
			WriteErrorResponse(w, http.StatusUnauthorized, "chain-id required but not specified")
			return false

		case !br.Fees.IsZero() && !br.GasPrices.IsZero():
			// both fees and gas prices were provided
			WriteErrorResponse(w, http.StatusBadRequest, "cannot provide both fees and gas prices")
			return false

		case !br.Fees.IsValid() && !br.GasPrices.IsValid():
			// neither fees or gas prices were provided
			WriteErrorResponse(w, http.StatusPaymentRequired, "invalid fees or gas prices provided")
			return false
		}
	}

	if len(br.From) == 0 {
		WriteErrorResponse(w, http.StatusUnauthorized, "name or address required but not specified")
		return false
	}

	return true
}

/*
ReadRESTReq is a simple convenience wrapper that reads the body and
unmarshals to the req interface. Returns false if errors occurred.

  Usage:
    type SomeReq struct {
      BaseReq            `json:"base_req"`
      CustomField string `json:"custom_field"`
		}

    req := new(SomeReq)
    if ok := ReadRESTReq(w, r, cdc, req); !ok {
        return
    }
*/func ReadRESTReq(w http.ResponseWriter, r *http.Request, cdc *codec.Codec, req interface{}) bool {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, err.Error())
		return false
	}

	err = cdc.UnmarshalJSON(body, req)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("failed to decode JSON payload: %s", err))
		return false
	}

	return true
}

// AddrSeed combines an Address with the mnemonic of the private key to that address
type AddrSeed struct {
	Address  types.AccAddress
	Seed     string
	Name     string
	Password string
}

// SendReq requests sending an amount of coins
type SendReq struct {
	Amount  types.Coins `json:"amount"`
	BaseReq BaseReq     `json:"base_req"`
}

// MsgBeginRedelegateInput request to begin a redelegation
type MsgBeginRedelegateInput struct {
	BaseReq          BaseReq          `json:"base_req"`
	DelegatorAddr    types.AccAddress `json:"delegator_addr"`     // in bech32
	ValidatorSrcAddr types.ValAddress `json:"validator_src_addr"` // in bech32
	ValidatorDstAddr types.ValAddress `json:"validator_dst_addr"` // in bech32
	SharesAmount     types.Dec        `json:"shares"`
}

// PostProposalReq requests a proposals
type PostProposalReq struct {
	BaseReq        BaseReq          `json:"base_req"`
	Title          string           `json:"title"`           //  Title of the proposal
	Description    string           `json:"description"`     //  Description of the proposal
	ProposalType   string           `json:"proposal_type"`   //  Type of proposal. Initial set {PlainTextProposal, SoftwareUpgradeProposal}
	Proposer       types.AccAddress `json:"proposer"`        //  Address of the proposer
	InitialDeposit types.Coins      `json:"initial_deposit"` // Coins to add to the proposal's deposit
}

// DepositReq requests a deposit of an amount of coins
type DepositReq struct {
	BaseReq   BaseReq          `json:"base_req"`
	Depositor types.AccAddress `json:"depositor"` // Address of the depositor
	Amount    types.Coins      `json:"amount"`    // Coins to add to the proposal's deposit
}

// VoteReq requests sending a vote
type VoteReq struct {
	BaseReq BaseReq          `json:"base_req"`
	Voter   types.AccAddress `json:"voter"`  //  address of the voter
	Option  string           `json:"option"` //  option from OptionSet chosen by the voter
}

// ErrorResponse defines the attributes of a JSON error response.
type ErrorResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

// NewErrorResponse creates a new ErrorResponse instance.
func NewErrorResponse(code int, msg string) ErrorResponse {
	return ErrorResponse{Code: code, Message: msg}
}

// WriteErrorResponse prepares and writes a HTTP error
// given a status code and an error message.
func WriteErrorResponse(w http.ResponseWriter, status int, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(codec.Cdc.MustMarshalJSON(NewErrorResponse(0, err)))
}

// WriteSimulationResponse prepares and writes an HTTP
// response for transactions simulations.
func WriteSimulationResponse(w http.ResponseWriter, cdc *codec.Codec, gas uint64) {
	gasEst := GasEstimateResponse{GasEstimate: gas}
	resp, err := cdc.MarshalJSON(gasEst)
	if err != nil {
		WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}

// ParseInt64OrReturnBadRequest converts s to a int64 value.
func ParseInt64OrReturnBadRequest(w http.ResponseWriter, s string) (n int64, ok bool) {
	var err error

	n, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		err := fmt.Errorf("'%s' is not a valid int64", s)
		WriteErrorResponse(w, http.StatusBadRequest, err.Error())
		return n, false
	}

	return n, true
}

// ParseUint64OrReturnBadRequest converts s to a uint64 value.
func ParseUint64OrReturnBadRequest(w http.ResponseWriter, s string) (n uint64, ok bool) {
	var err error

	n, err = strconv.ParseUint(s, 10, 64)
	if err != nil {
		err := fmt.Errorf("'%s' is not a valid uint64", s)
		WriteErrorResponse(w, http.StatusBadRequest, err.Error())
		return n, false
	}

	return n, true
}

// ParseFloat64OrReturnBadRequest converts s to a float64 value. It returns a
// default value, defaultIfEmpty, if the string is empty.
func ParseFloat64OrReturnBadRequest(w http.ResponseWriter, s string, defaultIfEmpty float64) (n float64, ok bool) {
	if len(s) == 0 {
		return defaultIfEmpty, true
	}

	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, err.Error())
		return n, false
	}

	return n, true
}

// PostProcessResponse performs post processing for a REST response.
func PostProcessResponse(w http.ResponseWriter, cdc *codec.Codec, response interface{}, indent bool) {
	var output []byte

	switch response.(type) {
	default:
		var err error
		if indent {
			output, err = cdc.MarshalJSONIndent(response, "", "  ")
		} else {
			output, err = cdc.MarshalJSON(response)
		}
		if err != nil {
			WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	case []byte:
		output = response.([]byte)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(output)
}