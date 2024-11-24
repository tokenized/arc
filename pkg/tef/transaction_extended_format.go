package tef

import (
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/expanded_tx"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

const (
	MarkerLockTime = uint32(0xef000000)

	txProtocolVersion = uint32(0)
)

var (
	Marker = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0xef}

	endian = binary.LittleEndian
)

func Serialize(w io.Writer, tx expanded_tx.TransactionWithOutputs) error {
	msgTx := tx.GetMsgTx()
	if err := binary.Write(w, endian, msgTx.Version); err != nil {
		return errors.Wrap(err, "version")
	}

	inputCount := tx.InputCount()
	if inputCount > 0 {
		// There is no need for extended format if there are no inputs.
		if _, err := w.Write(Marker); err != nil {
			return errors.Wrap(err, "marker")
		}
	}

	if err := wire.WriteVarInt(w, txProtocolVersion, uint64(inputCount)); err != nil {
		return errors.Wrap(err, "input count")
	}

	for inputIndex := 0; inputIndex < inputCount; inputIndex++ {
		input := tx.Input(inputIndex)

		if input.PreviousOutPoint.Hash.IsZero() { // coinbase input
			if err := SerializeExtendedInput(w, input, &wire.TxOut{}); err != nil {
				return errors.Wrapf(err, "input %d", inputIndex)
			}
		}

		inputOutput, err := tx.InputOutput(inputIndex)
		if err != nil {
			return errors.Wrapf(err, "input output %d", inputIndex)
		}

		if err := SerializeExtendedInput(w, input, inputOutput); err != nil {
			return errors.Wrapf(err, "input %d", inputIndex)
		}
	}

	outputCount := tx.OutputCount()
	if err := wire.WriteVarInt(w, txProtocolVersion, uint64(outputCount)); err != nil {
		return errors.Wrap(err, "output count")
	}

	for outputIndex := 0; outputIndex < outputCount; outputIndex++ {
		output := tx.Output(outputIndex)
		if err := output.Serialize(w, txProtocolVersion, 0); err != nil {
			return errors.Wrapf(err, "output %d", outputIndex)
		}
	}

	if err := binary.Write(w, endian, uint32(msgTx.LockTime)); err != nil {
		return errors.Wrap(err, "lock time")
	}

	return nil
}

func SerializeExtendedInput(w io.Writer, input *wire.TxIn, output *wire.TxOut) error {
	if err := input.Serialize(w, txProtocolVersion, 0); err != nil {
		return errors.Wrap(err, "input")
	}

	if err := output.Serialize(w, txProtocolVersion, 0); err != nil {
		return errors.Wrap(err, "output")
	}

	return nil
}

func Deserialize(r io.Reader) (*expanded_tx.ExpandedTx, error) {
	msgTx := &wire.MsgTx{}

	if err := binary.Read(r, endian, &msgTx.Version); err != nil {
		return nil, errors.Wrap(err, "version")
	}

	inputCount, err := wire.ReadVarInt(r, txProtocolVersion)
	if err != nil {
		return nil, errors.Wrap(err, "input count")
	}

	if inputCount > 0 {
		// The tx isn't extended.
		msgTx.TxIn = make([]*wire.TxIn, inputCount)
		for inputIndex := range msgTx.TxIn {
			txin := &wire.TxIn{}
			if err := txin.Deserialize(r, txProtocolVersion, 0); err != nil {
				return nil, errors.Wrapf(err, "input %d", inputIndex)
			}

			msgTx.TxIn[inputIndex] = txin
		}

		outputCount, err := wire.ReadVarInt(r, txProtocolVersion)
		if err != nil {
			return nil, errors.Wrap(err, "input count")
		}

		msgTx.TxOut = make([]*wire.TxOut, outputCount)
		for outputIndex := range msgTx.TxOut {
			txout := &wire.TxOut{}
			if err := txout.Deserialize(r, txProtocolVersion, 0); err != nil {
				return nil, errors.Wrapf(err, "output %d", outputIndex)
			}

			msgTx.TxOut[outputIndex] = txout
		}

		if err := binary.Read(r, endian, &msgTx.LockTime); err != nil {
			return nil, errors.Wrap(err, "lock time")
		}

		return &expanded_tx.ExpandedTx{
			Tx: msgTx,
		}, nil
	}

	outputCount, err := wire.ReadVarInt(r, txProtocolVersion)
	if err != nil {
		return nil, errors.Wrap(err, "input count")
	}

	if outputCount > 0 {
		// The tx doesn't have any inputs, but isn't extended.
		msgTx.TxOut = make([]*wire.TxOut, outputCount)
		for outputIndex := range msgTx.TxOut {
			txout := &wire.TxOut{}
			if err := txout.Deserialize(r, txProtocolVersion, 0); err != nil {
				return nil, errors.Wrapf(err, "output %d", outputIndex)
			}

			msgTx.TxOut[outputIndex] = txout
		}

		if err := binary.Read(r, endian, &msgTx.LockTime); err != nil {
			return nil, errors.Wrap(err, "lock time")
		}

		return &expanded_tx.ExpandedTx{
			Tx: msgTx,
		}, nil
	}

	if err := binary.Read(r, endian, &msgTx.LockTime); err != nil {
		return nil, errors.Wrap(err, "lock time")
	}

	if msgTx.LockTime != MarkerLockTime {
		// The tx has no inputs or outputs, but isn't extended.
		return &expanded_tx.ExpandedTx{
			Tx: msgTx,
		}, nil
	}

	// The tx is extended and the actual input count follows.
	count, err := wire.ReadVarInt(r, txProtocolVersion)
	if err != nil {
		return nil, errors.Wrap(err, "actual input count")
	}
	inputCount = count

	spentOutputs := make(expanded_tx.Outputs, inputCount)
	msgTx.TxIn = make([]*wire.TxIn, inputCount)
	for inputIndex := range msgTx.TxIn {
		input := &wire.TxIn{}
		output := &wire.TxOut{}
		if err := DeserializeExtendedOutput(r, input, output); err != nil {
			return nil, errors.Wrapf(err, "input %d", inputIndex)
		}

		msgTx.TxIn[inputIndex] = input
		spentOutputs[inputIndex] = &expanded_tx.Output{
			Value:         output.Value,
			LockingScript: output.LockingScript,
		}
	}

	if err := readOutputs(r, msgTx); err != nil {
		return nil, errors.Wrap(err, "read non extended outputs")
	}

	if err := binary.Read(r, endian, &msgTx.LockTime); err != nil {
		return nil, errors.Wrap(err, "lock time")
	}

	return &expanded_tx.ExpandedTx{
		Tx:           msgTx,
		SpentOutputs: spentOutputs,
	}, nil
}

func DeserializeTxID(r io.Reader) (bitcoin.Hash32, error) {
	hasher := sha256.New()
	tee := io.TeeReader(r, hasher)

	var version int32
	if err := binary.Read(tee, endian, &version); err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "version")
	}

	inputCount, err := wire.ReadVarInt(r, txProtocolVersion)
	if err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "input count")
	}

	if inputCount > 0 {
		// The tx isn't extended.
		if err := wire.WriteVarInt(hasher, txProtocolVersion, inputCount); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "write input count")
		}

		for inputIndex := uint64(0); inputIndex < inputCount; inputIndex++ {
			txin := &wire.TxIn{}
			if err := txin.Deserialize(tee, txProtocolVersion, 0); err != nil {
				return bitcoin.Hash32{}, errors.Wrapf(err, "input %d", inputIndex)
			}
		}

		outputCount, err := wire.ReadVarInt(tee, txProtocolVersion)
		if err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "input count")
		}

		for outputIndex := uint64(0); outputIndex < outputCount; outputIndex++ {
			txout := &wire.TxOut{}
			if err := txout.Deserialize(tee, txProtocolVersion, 0); err != nil {
				return bitcoin.Hash32{}, errors.Wrapf(err, "output %d", outputIndex)
			}
		}

		var lockTime uint32
		if err := binary.Read(tee, endian, &lockTime); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "lock time")
		}

		return bitcoin.Hash32(sha256.Sum256(hasher.Sum(nil))), nil
	}

	outputCount, err := wire.ReadVarInt(r, txProtocolVersion)
	if err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "input count")
	}

	if outputCount > 0 {
		// The tx doesn't have any inputs, but isn't extended.
		if err := wire.WriteVarInt(hasher, txProtocolVersion, inputCount); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "write input count")
		}

		if err := wire.WriteVarInt(hasher, txProtocolVersion, outputCount); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "write output count")
		}

		for outputIndex := uint64(0); outputIndex < outputCount; outputIndex++ {
			txout := &wire.TxOut{}
			if err := txout.Deserialize(tee, txProtocolVersion, 0); err != nil {
				return bitcoin.Hash32{}, errors.Wrapf(err, "output %d", outputIndex)
			}
		}

		var lockTime uint32
		if err := binary.Read(tee, endian, &lockTime); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "lock time")
		}

		return bitcoin.Hash32(sha256.Sum256(hasher.Sum(nil))), nil
	}

	var lockTime uint32
	if err := binary.Read(r, endian, &lockTime); err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "lock time")
	}

	if lockTime != MarkerLockTime {
		// The tx has no inputs or outputs, but isn't extended.
		if err := wire.WriteVarInt(hasher, txProtocolVersion, inputCount); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "write input count")
		}

		if err := wire.WriteVarInt(hasher, txProtocolVersion, outputCount); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "write output count")
		}

		if err := binary.Write(hasher, endian, lockTime); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "lock time")
		}

		return bitcoin.Hash32(sha256.Sum256(hasher.Sum(nil))), nil
	}

	// The tx is extended and the actual input count follows.
	icount, err := wire.ReadVarInt(tee, txProtocolVersion)
	if err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "actual input count")
	}
	inputCount = icount

	for inputIndex := uint64(0); inputIndex < inputCount; inputIndex++ {
		input := &wire.TxIn{}
		if err := input.Deserialize(tee, txProtocolVersion, 0); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "input")
		}

		output := &wire.TxOut{}
		if err := output.Deserialize(r, txProtocolVersion, 0); err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "output")
		}
	}

	ocount, err := wire.ReadVarInt(tee, txProtocolVersion)
	if err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "input count")
	}
	outputCount = ocount

	for outputIndex := uint64(0); outputIndex < outputCount; outputIndex++ {
		txout := &wire.TxOut{}
		if err := txout.Deserialize(tee, txProtocolVersion, 0); err != nil {
			return bitcoin.Hash32{}, errors.Wrapf(err, "output %d", outputIndex)
		}
	}

	if err := binary.Read(tee, endian, &lockTime); err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "lock time")
	}

	return bitcoin.Hash32(sha256.Sum256(hasher.Sum(nil))), nil
}

func DeserializeExtendedOutput(r io.Reader, input *wire.TxIn, output *wire.TxOut) error {
	if err := input.Deserialize(r, txProtocolVersion, 0); err != nil {
		return errors.Wrap(err, "input")
	}

	if err := output.Deserialize(r, txProtocolVersion, 0); err != nil {
		return errors.Wrap(err, "output")
	}

	return nil
}

func readOutputs(r io.Reader, msgTx *wire.MsgTx) error {
	outputCount, err := wire.ReadVarInt(r, txProtocolVersion)
	if err != nil {
		return errors.Wrap(err, "input count")
	}

	msgTx.TxOut = make([]*wire.TxOut, outputCount)
	for outputIndex := range msgTx.TxOut {
		txout := &wire.TxOut{}
		if err := txout.Deserialize(r, txProtocolVersion, 0); err != nil {
			return errors.Wrapf(err, "output %d", outputIndex)
		}

		msgTx.TxOut[outputIndex] = txout
	}

	return nil
}
