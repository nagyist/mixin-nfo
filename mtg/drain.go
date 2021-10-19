package mtg

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/fox-one/mixin-sdk-go"
)

const (
	outputsDrainingKey            = "outputs-draining-checkpoint"
	collectibleOutputsDrainingKey = "collectible-outputs-draining-checkpoint"
)

func (grp *Group) drainOutputsFromNetwork(ctx context.Context, batch int) {
	for {
		checkpoint, err := grp.readOutputsDrainingCheckpoint(ctx)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		outputs, err := grp.mixin.ReadMultisigOutputs(ctx, grp.members, uint8(grp.threshold), checkpoint, batch)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		checkpoint = grp.processMultisigOutputs(checkpoint, outputs)

		grp.writeOutputsDrainingCheckpoint(ctx, checkpoint)
		if len(outputs) < batch/2 {
			break
		}
	}
}

func (grp *Group) processMultisigOutputs(checkpoint time.Time, outputs []*mixin.MultisigUTXO) time.Time {
	for _, out := range outputs {
		checkpoint = out.UpdatedAt
		ver, extra := decodeTransactionWithExtra(out.SignedTx)
		if out.SignedTx != "" && ver == nil {
			panic(out.SignedTx)
		}
		if out.State == mixin.UTXOStateUnspent {
			grp.writeOutput(out, "", nil)
			continue
		}
		tx := &Transaction{
			TraceId: extra.T.String(),
			State:   TransactionStateInitial,
			Raw:     ver.Marshal(),
		}
		if ver.AggregatedSignature != nil {
			out.State = mixin.UTXOStateSpent
			tx.State = TransactionStateSigned
		}
		grp.writeOutput(out, tx.TraceId, tx)
	}

	for _, utxo := range outputs {
		out := NewOutputFromMultisig(utxo)
		grp.writeAction(out, ActionStateInitial)
	}
	return checkpoint
}

func (grp *Group) writeOutput(utxo *mixin.MultisigUTXO, traceId string, tx *Transaction) {
	out := NewOutputFromMultisig(utxo)
	err := grp.store.WriteOutput(out, traceId)
	if err != nil {
		panic(err)
	}
	if traceId == "" {
		return
	}
	old, err := grp.store.ReadTransaction(traceId)
	if err != nil {
		panic(err)
	}
	if old != nil && old.State >= TransactionStateSigned {
		return
	}
	err = grp.store.WriteTransaction(traceId, tx)
	if err != nil {
		panic(err)
	}
}

func (grp *Group) readOutputsDrainingCheckpoint(ctx context.Context) (time.Time, error) {
	key := []byte(outputsDrainingKey)
	val, err := grp.store.ReadProperty(key)
	if err != nil || len(val) == 0 {
		return time.Time{}, nil
	}
	ts := int64(binary.BigEndian.Uint64(val))
	return time.Unix(0, ts), nil
}

func (grp *Group) writeOutputsDrainingCheckpoint(ctx context.Context, ckpt time.Time) error {
	val := make([]byte, 8)
	key := []byte(outputsDrainingKey)
	ts := uint64(ckpt.UnixNano())
	binary.BigEndian.PutUint64(val, ts)
	return grp.store.WriteProperty(key, val)
}

func (grp *Group) drainCollectibleOutputsFromNetwork(ctx context.Context, batch int) {
	for {
		checkpoint, err := grp.readCollectibleOutputsDrainingCheckpoint(ctx)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		outputs, err := grp.ReadCollectibleOutputs(ctx, grp.members, uint8(grp.threshold), checkpoint, batch)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		checkpoint = grp.processCollectibleOutputs(checkpoint, outputs)

		grp.writeCollectibleOutputsDrainingCheckpoint(ctx, checkpoint)
		if len(outputs) < batch/2 {
			break
		}
	}
}

func (grp *Group) processCollectibleOutputs(checkpoint time.Time, outputs []*CollectibleOutput) time.Time {
	for _, out := range outputs {
		checkpoint = out.UpdatedAt
		ver, extra := decodeTransactionWithExtra(out.SignedTx)
		if out.SignedTx != "" && ver == nil {
			panic(out.SignedTx)
		}
		if out.State == OutputStateUnspent {
			grp.writeCollectibleOutput(out, "", nil)
			continue
		}
		tx := &Transaction{
			TraceId: extra.T.String(),
			State:   TransactionStateInitial,
			Raw:     ver.Marshal(),
		}
		if ver.AggregatedSignature != nil {
			out.State = OutputStateSpent
			tx.State = TransactionStateSigned
		}
		grp.writeCollectibleOutput(out, tx.TraceId, tx)
	}

	return checkpoint
}

func (grp *Group) writeCollectibleOutput(out *CollectibleOutput, traceId string, tx *Transaction) {
	err := grp.store.WriteCollectibleOutput(out, traceId)
	if err != nil {
		panic(err)
	}
	if traceId == "" {
		return
	}
	old, err := grp.store.ReadCollectibleTransaction(traceId)
	if err != nil {
		panic(err)
	}
	if old != nil && old.State >= TransactionStateSigned {
		return
	}
	err = grp.store.WriteCollectibleTransaction(traceId, tx)
	if err != nil {
		panic(err)
	}
}

func (grp *Group) readCollectibleOutputsDrainingCheckpoint(ctx context.Context) (time.Time, error) {
	key := []byte(collectibleOutputsDrainingKey)
	val, err := grp.store.ReadProperty(key)
	if err != nil || len(val) == 0 {
		return time.Time{}, nil
	}
	ts := int64(binary.BigEndian.Uint64(val))
	return time.Unix(0, ts), nil
}

func (grp *Group) writeCollectibleOutputsDrainingCheckpoint(ctx context.Context, ckpt time.Time) error {
	val := make([]byte, 8)
	key := []byte(collectibleOutputsDrainingKey)
	ts := uint64(ckpt.UnixNano())
	binary.BigEndian.PutUint64(val, ts)
	return grp.store.WriteProperty(key, val)
}
