package arc

import (
	"context"
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/expanded_tx"

	"github.com/pkg/errors"
)

// https://github.com/bitcoin-sv/arc/blob/main/README.md

const (
	// TxStatusUnknown - The transaction has been sent to metamorph, but no processing has taken
	// place. This should never be the case, unless something goes wrong.
	TxStatusUnknown = TxStatus(0)

	// TxStatusQueued - The transaction has been queued for processing.
	TxStatusQueued = TxStatus(1)

	// TxStatusReceived -The transaction has been properly received by the metamorph processor.
	TxStatusReceived = TxStatus(2) // integer value corresponds to the X-WaitForStatus

	// TxStatusStored -The transaction has been stored in the metamorph store. This should ensure
	// the transaction will be processed and retried if not picked up immediately by a mining node.
	TxStatusStored = TxStatus(3) // integer value corresponds to the X-WaitForStatus

	// TxStatusAnnounced - The transaction has been announced (INV message) to the Bitcoin network.
	TxStatusAnnounced = TxStatus(4) // integer value corresponds to the X-WaitForStatus

	// TxStatusRequested - The transaction has been requested from metamorph by a Bitcoin node.
	TxStatusRequested = TxStatus(5) // integer value corresponds to the X-WaitForStatus

	// TxStatusSent - The transaction has been sent to at least 1 Bitcoin node.
	TxStatusSent = TxStatus(6) // integer value corresponds to the X-WaitForStatus

	// TxStatusAccepted - The transaction has been accepted by a connected Bitcoin node on the ZMQ
	// interface. If metamorph is not connected to ZMQ, this status will never by set.
	TxStatusAccepted = TxStatus(7) // integer value corresponds to the X-WaitForStatus

	// TxStatusSeen - The transaction has been seen on the Bitcoin network and propagated to other
	// nodes. This status is set when metamorph receives an INV message for the transaction from
	// another node than it was sent to.
	TxStatusSeen = TxStatus(8) // integer value corresponds to the X-WaitForStatus

	// TxStatusMined - The transaction has been mined into a block by a mining node.
	TxStatusMined = TxStatus(9)

	// TxStatusOrphaned - The transaction has been sent to at least 1 Bitcoin node but parent
	// transaction was not found.
	// This might be seen with txs that are chained too quickly, but should quickly be updated to
	// "seen".
	TxStatusOrphaned = TxStatus(10)

	// TxStatusConfirmed - The transaction is marked as confirmed when it is in a block with 100
	// blocks built on top of that block.
	TxStatusConfirmed = TxStatus(108)

	// TxStatusRejected - The transaction has been rejected by the Bitcoin network.
	TxStatusRejected = TxStatus(109)
)

var (
	ErrTimeout         = errors.New("Timeout")
	ErrInvalidTxStatus = errors.New("Invalid Tx Status")
)

type TxStatus uint32

type Client interface {
	URL() string
	GetPolicy(context.Context) (*Policy, error)
	GetTxStatus(context.Context, bitcoin.Hash32) (*TxStatusResponse, error)
	SubmitTx(context.Context, expanded_tx.TransactionWithOutputs) (*TxSubmitResponse, error)
	SubmitTxBytes(context.Context, []byte) (*TxSubmitResponse, error)
	SubmitTxs(context.Context, []expanded_tx.TransactionWithOutputs) ([]*TxSubmitResponse, error)
	SubmitTxsBytes(context.Context, []byte) ([]*TxSubmitResponse, error)
}

type MiningFee struct {
	Satoshis uint64 `json:"satoshis"`
	Bytes    uint64 `json:"bytes"`
}

func (f MiningFee) Rate() float64 {
	return float64(f.Satoshis) / float64(f.Bytes)
}

type PolicyData struct {
	MaxScriptSize    int       `json:"maxscriptsizepolicy"`
	MaxTxSigOpsCount int       `json:"maxtxsigopscountspolicy"`
	MaxTxSize        int       `json:"maxtxsizepolicy"`
	MiningFee        MiningFee `json:"miningFee"`
}

type Policy struct {
	Timestamp time.Time  `json:"timestamp"`
	Policy    PolicyData `json:"policy"`
}

type TxStatusResponse struct {
	Timestamp   time.Time      `json:"timestamp"`
	BlockHash   bitcoin.Hash32 `json:"blockHash"`
	BlockHeight int            `json:"blockHeight"`
	TxID        bitcoin.Hash32 `json:"txid"`
	MerklePath  *string        `json:"merklePath,omitempty"` // https://bsv.brc.dev/transactions/0074
	TxStatus    TxStatus       `json:"txStatus"`
	ExtraInfo   *string        `json:"extraInfo,omitempty"`
}

type TxSubmitResponse struct {
	Timestamp   time.Time      `json:"timestamp"`
	BlockHash   bitcoin.Hash32 `json:"blockHash"`
	BlockHeight int            `json:"blockHeight"`
	Status      int            `json:"status"`
	Title       string         `json:"title"`
	TxID        bitcoin.Hash32 `json:"txid"`
	MerklePath  *string        `json:"merklePath,omitempty"` // https://bsv.brc.dev/transactions/0074
	TxStatus    TxStatus       `json:"txStatus"`
	ExtraInfo   *string        `json:"extraInfo,omitempty"`
}

type ErrorData struct {
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Status    int             `json:"status"`
	Detail    string          `json:"detail"`
	Instance  *string         `json:"instance,omitempty"`
	TxID      *bitcoin.Hash32 `json:"txid,omitempty"`
	ExtraInfo *string         `json:"extraInfo,omitempty"`
}

type Callback struct {
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Status    int             `json:"status"`
	Detail    string          `json:"detail"`
	Instance  *string         `json:"instance,omitempty"`
	TxID      *bitcoin.Hash32 `json:"txid,omitempty"`
	ExtraInfo *string         `json:"extraInfo,omitempty"`

	Timestamp   *time.Time      `json:"timestamp,omitempty"`
	BlockHash   *bitcoin.Hash32 `json:"blockHash,omitempty"`
	BlockHeight int             `json:"blockHeight,omitempty"`
	MerklePath  *string         `json:"merklePath,omitempty"` // https://bsv.brc.dev/transactions/0074
	TxStatus    *TxStatus       `json:"txStatus,omitempty"`
}

func (s TxStatus) String() string {
	switch s {
	case TxStatusUnknown:
		return "UNKNOWN"
	case TxStatusQueued:
		return "QUEUED"
	case TxStatusReceived:
		return "RECEIVED"
	case TxStatusStored:
		return "STORED"
	case TxStatusAnnounced:
		return "ANNOUNCED_TO_NETWORK"
	case TxStatusRequested:
		return "REQUESTED_BY_NETWORK"
	case TxStatusSent:
		return "SENT_TO_NETWORK"
	case TxStatusAccepted:
		return "ACCEPTED_BY_NETWORK"
	case TxStatusSeen:
		return "SEEN_ON_NETWORK"
	case TxStatusMined:
		return "MINED"
	case TxStatusConfirmed:
		return "CONFIRMED"
	case TxStatusRejected:
		return "REJECTED"
	case TxStatusOrphaned:
		return "SEEN_IN_ORPHAN_MEMPOOL"
	default:
		return ""
	}
}

func (s *TxStatus) SetString(v string) error {
	switch v {
	case "UNKNOWN":
		*s = TxStatusUnknown
	case "QUEUED":
		*s = TxStatusQueued
	case "RECEIVED":
		*s = TxStatusReceived
	case "STORED":
		*s = TxStatusStored
	case "ANNOUNCED_TO_NETWORK":
		*s = TxStatusAnnounced
	case "REQUESTED_BY_NETWORK":
		*s = TxStatusRequested
	case "SENT_TO_NETWORK":
		*s = TxStatusSent
	case "ACCEPTED_BY_NETWORK":
		*s = TxStatusAccepted
	case "SEEN_ON_NETWORK":
		*s = TxStatusSeen
	case "MINED":
		*s = TxStatusMined
	case "CONFIRMED":
		*s = TxStatusConfirmed
	case "REJECTED":
		*s = TxStatusRejected
	case "SEEN_IN_ORPHAN_MEMPOOL":
		*s = TxStatusOrphaned
	default:
		*s = TxStatusUnknown
		return errors.Wrap(ErrInvalidTxStatus, v)
	}

	return nil
}

func (s TxStatus) MarshalText() (text []byte, err error) {
	return []byte(s.String()), nil
}

func (s *TxStatus) UnmarshalText(text []byte) error {
	return s.SetString(string(text))
}
