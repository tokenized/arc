package tef

import (
	"bytes"
	"testing"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/expanded_tx"
	"github.com/tokenized/pkg/wire"
)

func Test_Serialize_Extended_WithInputsAndOutputs(t *testing.T) {
	inputTx := wire.NewMsgTx(1)
	inputKey, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	inputLockingScript, _ := inputKey.LockingScript()
	inputValue := uint64(10000)
	inputTx.AddTxOut(wire.NewTxOut(inputValue, inputLockingScript))

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(inputTx.TxHash(), 0), nil))
	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	lockingScript, _ := key.LockingScript()
	tx.AddTxOut(wire.NewTxOut(9990, lockingScript))
	txid := *tx.TxHash()

	t.Logf("Input tx : %s", inputTx)
	t.Logf("Tx : %s", tx)

	etx := &expanded_tx.ExpandedTx{
		Tx: tx,
		Ancestors: expanded_tx.AncestorTxs{
			{
				Tx: inputTx,
			},
		},
	}

	buf := &bytes.Buffer{}
	if err := Serialize(buf, etx); err != nil {
		t.Fatalf("Failed to serialize extended format : %s", err)
	}

	tefBytes := buf.Bytes()
	t.Logf("TEF bytes : %x", tefBytes)

	detx, err := Deserialize(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize extended format : %s", err)
	}

	if len(detx.SpentOutputs) != 1 {
		t.Fatalf("Wrong deserialized ancestor count : got %d, want %d", len(detx.SpentOutputs), 1)
	}

	t.Logf("Deserialized input tx : %s", detx.SpentOutputs[0])
	t.Logf("Deserialized tx : %s", detx.Tx)

	if !detx.SpentOutputs[0].LockingScript.Equal(inputLockingScript) {
		t.Fatalf("Wrong input locking script : \n   got %s\n  want %s",
			detx.SpentOutputs[0].LockingScript, inputLockingScript)
	}

	if detx.SpentOutputs[0].Value != inputValue {
		t.Fatalf("Wrong input value : got %d, want %d", detx.SpentOutputs[0].Value, inputValue)
	}

	detxid := *detx.Tx.TxHash()
	if !detxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized tx txid : \n   got %s\n  want %s", detxid, txid)
	}

	dtxid, err := DeserializeTxID(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize txid : %s", err)
	}

	if !dtxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized txid : \n   got %s\n  want %s", dtxid, txid)
	}
}

func Test_Serialize_Extended_WithZeroInputs(t *testing.T) {
	tx := wire.NewMsgTx(1)
	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	lockingScript, _ := key.LockingScript()
	tx.AddTxOut(wire.NewTxOut(9990, lockingScript))
	txid := *tx.TxHash()

	t.Logf("Tx : %s", tx)

	etx := &expanded_tx.ExpandedTx{
		Tx: tx,
	}

	buf := &bytes.Buffer{}
	if err := Serialize(buf, etx); err != nil {
		t.Fatalf("Failed to serialize extended format : %s", err)
	}

	tefBytes := buf.Bytes()
	t.Logf("TEF bytes : %x", tefBytes)

	detx, err := Deserialize(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize extended format : %s", err)
	}

	if len(detx.SpentOutputs) != 0 {
		t.Fatalf("Wrong deserialized ancestor count : got %d, want %d", len(detx.SpentOutputs), 0)
	}

	t.Logf("Deserialized tx : %s", detx.Tx)

	detxid := *detx.Tx.TxHash()
	if !detxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized tx txid : \n   got %s\n  want %s", detxid, txid)
	}

	dtxid, err := DeserializeTxID(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize txid : %s", err)
	}

	if !dtxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized txid : \n   got %s\n  want %s", dtxid, txid)
	}
}

func Test_Serialize_Extended_WithZeroInputsAndZeroOutputs(t *testing.T) {
	tx := wire.NewMsgTx(1)
	txid := *tx.TxHash()

	t.Logf("Tx : %s", tx)

	etx := &expanded_tx.ExpandedTx{
		Tx: tx,
	}

	buf := &bytes.Buffer{}
	if err := Serialize(buf, etx); err != nil {
		t.Fatalf("Failed to serialize extended format : %s", err)
	}

	tefBytes := buf.Bytes()
	t.Logf("TEF bytes : %x", tefBytes)

	detx, err := Deserialize(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize extended format : %s", err)
	}

	if len(detx.SpentOutputs) != 0 {
		t.Fatalf("Wrong deserialized ancestor count : got %d, want %d", len(detx.SpentOutputs), 0)
	}

	t.Logf("Deserialized tx : %s", detx.Tx)

	detxid := *detx.Tx.TxHash()
	if !detxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized tx txid : \n   got %s\n  want %s", detxid, txid)
	}

	dtxid, err := DeserializeTxID(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize txid : %s", err)
	}

	if !dtxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized txid : \n   got %s\n  want %s", dtxid, txid)
	}
}

func Test_Serialize_NotExtended_WithInputsAndOutputs(t *testing.T) {
	inputTx := wire.NewMsgTx(1)
	inputKey, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	inputLockingScript, _ := inputKey.LockingScript()
	inputValue := uint64(10000)
	inputTx.AddTxOut(wire.NewTxOut(inputValue, inputLockingScript))

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(inputTx.TxHash(), 0), nil))
	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	lockingScript, _ := key.LockingScript()
	tx.AddTxOut(wire.NewTxOut(9990, lockingScript))
	txid := *tx.TxHash()

	t.Logf("Input tx : %s", inputTx)
	t.Logf("Tx : %s", tx)

	buf := &bytes.Buffer{}
	if err := tx.Serialize(buf); err != nil {
		t.Fatalf("Failed to serialize extended format : %s", err)
	}

	tefBytes := buf.Bytes()
	t.Logf("TEF bytes : %x", tefBytes)

	detx, err := Deserialize(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize extended format : %s", err)
	}

	if len(detx.SpentOutputs) != 0 {
		t.Fatalf("Wrong deserialized ancestor count : got %d, want %d", len(detx.SpentOutputs), 0)
	}

	t.Logf("Deserialized tx : %s", detx.Tx)

	detxid := *detx.Tx.TxHash()
	if !detxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized tx txid : \n   got %s\n  want %s", detxid, txid)
	}

	dtxid, err := DeserializeTxID(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize txid : %s", err)
	}

	if !dtxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized txid : \n   got %s\n  want %s", dtxid, txid)
	}
}

func Test_Serialize_NotExtended_WithZeroInputs(t *testing.T) {
	tx := wire.NewMsgTx(1)
	key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
	lockingScript, _ := key.LockingScript()
	tx.AddTxOut(wire.NewTxOut(9990, lockingScript))
	txid := *tx.TxHash()

	t.Logf("Tx : %s", tx)

	buf := &bytes.Buffer{}
	if err := tx.Serialize(buf); err != nil {
		t.Fatalf("Failed to serialize extended format : %s", err)
	}

	tefBytes := buf.Bytes()
	t.Logf("TEF bytes : %x", tefBytes)

	detx, err := Deserialize(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize extended format : %s", err)
	}

	if len(detx.SpentOutputs) != 0 {
		t.Fatalf("Wrong deserialized ancestor count : got %d, want %d", len(detx.SpentOutputs), 0)
	}

	t.Logf("Deserialized tx : %s", detx.Tx)

	detxid := *detx.Tx.TxHash()
	if !detxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized tx txid : \n   got %s\n  want %s", detxid, txid)
	}

	dtxid, err := DeserializeTxID(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize txid : %s", err)
	}

	if !dtxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized txid : \n   got %s\n  want %s", dtxid, txid)
	}
}

func Test_Serialize_NotExtended_WithZeroInputsAndZeroOutputs(t *testing.T) {
	tx := wire.NewMsgTx(1)
	txid := *tx.TxHash()

	t.Logf("Tx : %s", tx)

	buf := &bytes.Buffer{}
	if err := tx.Serialize(buf); err != nil {
		t.Fatalf("Failed to serialize extended format : %s", err)
	}

	tefBytes := buf.Bytes()
	t.Logf("TEF bytes : %x", tefBytes)

	detx, err := Deserialize(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize extended format : %s", err)
	}

	if len(detx.SpentOutputs) != 0 {
		t.Fatalf("Wrong deserialized ancestor count : got %d, want %d", len(detx.SpentOutputs), 0)
	}

	t.Logf("Deserialized tx : %s", detx.Tx)

	detxid := *detx.Tx.TxHash()
	if !detxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized tx txid : \n   got %s\n  want %s", detxid, txid)
	}

	dtxid, err := DeserializeTxID(bytes.NewReader(tefBytes))
	if err != nil {
		t.Fatalf("Failed to deserialize txid : %s", err)
	}

	if !dtxid.Equal(&txid) {
		t.Fatalf("Wrong deserialized txid : \n   got %s\n  want %s", dtxid, txid)
	}
}
