// AUTO GENERATED FILE (by membufc proto compiler v0.4.0)
package serializer

import (
	"bytes"
	"fmt"
	"github.com/orbs-network/membuffers/go"
	//"github.com/orbs-network/orbs-spec/interfaces/primitives"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
)

/////////////////////////////////////////////////////////////////////////////
// message SerializedContractKeyValueEntry

// reader

type SerializedContractKeyValueEntry struct {
	// ContractName primitives.ContractName
	// Key []byte
	// Value []byte

	// internal
	// implements membuffers.Message
	_message membuffers.InternalMessage
}

func (x *SerializedContractKeyValueEntry) String() string {
	if x == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{ContractName:%s,Key:%s,Value:%s,}", x.StringContractName(), x.StringKey(), x.StringValue())
}

var _SerializedContractKeyValueEntry_Scheme = []membuffers.FieldType{membuffers.TypeString, membuffers.TypeBytes, membuffers.TypeBytes}
var _SerializedContractKeyValueEntry_Unions = [][]membuffers.FieldType{}

func SerializedContractKeyValueEntryReader(buf []byte) *SerializedContractKeyValueEntry {
	x := &SerializedContractKeyValueEntry{}
	x._message.Init(buf, membuffers.Offset(len(buf)), _SerializedContractKeyValueEntry_Scheme, _SerializedContractKeyValueEntry_Unions)
	return x
}

func (x *SerializedContractKeyValueEntry) IsValid() bool {
	return x._message.IsValid()
}

func (x *SerializedContractKeyValueEntry) Raw() []byte {
	return x._message.RawBuffer()
}

func (x *SerializedContractKeyValueEntry) Equal(y *SerializedContractKeyValueEntry) bool {
	if x == nil && y == nil {
		return true
	}
	if x == nil || y == nil {
		return false
	}
	return bytes.Equal(x.Raw(), y.Raw())
}

func (x *SerializedContractKeyValueEntry) ContractName() primitives.ContractName {
	return primitives.ContractName(x._message.GetString(0))
}

func (x *SerializedContractKeyValueEntry) RawContractName() []byte {
	return x._message.RawBufferForField(0, 0)
}

func (x *SerializedContractKeyValueEntry) RawContractNameWithHeader() []byte {
	return x._message.RawBufferWithHeaderForField(0, 0)
}

func (x *SerializedContractKeyValueEntry) MutateContractName(v primitives.ContractName) error {
	return x._message.SetString(0, string(v))
}

func (x *SerializedContractKeyValueEntry) StringContractName() string {
	return fmt.Sprintf("%s", x.ContractName())
}

func (x *SerializedContractKeyValueEntry) Key() []byte {
	return x._message.GetBytes(1)
}

func (x *SerializedContractKeyValueEntry) RawKey() []byte {
	return x._message.RawBufferForField(1, 0)
}

func (x *SerializedContractKeyValueEntry) RawKeyWithHeader() []byte {
	return x._message.RawBufferWithHeaderForField(1, 0)
}

func (x *SerializedContractKeyValueEntry) MutateKey(v []byte) error {
	return x._message.SetBytes(1, v)
}

func (x *SerializedContractKeyValueEntry) StringKey() string {
	return fmt.Sprintf("%x", x.Key())
}

func (x *SerializedContractKeyValueEntry) Value() []byte {
	return x._message.GetBytes(2)
}

func (x *SerializedContractKeyValueEntry) RawValue() []byte {
	return x._message.RawBufferForField(2, 0)
}

func (x *SerializedContractKeyValueEntry) RawValueWithHeader() []byte {
	return x._message.RawBufferWithHeaderForField(2, 0)
}

func (x *SerializedContractKeyValueEntry) MutateValue(v []byte) error {
	return x._message.SetBytes(2, v)
}

func (x *SerializedContractKeyValueEntry) StringValue() string {
	return fmt.Sprintf("%x", x.Value())
}

// builder

type SerializedContractKeyValueEntryBuilder struct {
	ContractName primitives.ContractName
	Key          []byte
	Value        []byte

	// internal
	// implements membuffers.Builder
	_builder               membuffers.InternalBuilder
	_overrideWithRawBuffer []byte
}

func (w *SerializedContractKeyValueEntryBuilder) Write(buf []byte) (err error) {
	if w == nil {
		return
	}
	w._builder.NotifyBuildStart()
	defer w._builder.NotifyBuildEnd()
	defer func() {
		if r := recover(); r != nil {
			err = &membuffers.ErrBufferOverrun{}
		}
	}()
	if w._overrideWithRawBuffer != nil {
		return w._builder.WriteOverrideWithRawBuffer(buf, w._overrideWithRawBuffer)
	}
	w._builder.Reset()
	w._builder.WriteString(buf, string(w.ContractName))
	w._builder.WriteBytes(buf, w.Key)
	w._builder.WriteBytes(buf, w.Value)
	return nil
}

func (w *SerializedContractKeyValueEntryBuilder) HexDump(prefix string, offsetFromStart membuffers.Offset) (err error) {
	if w == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			err = &membuffers.ErrBufferOverrun{}
		}
	}()
	w._builder.Reset()
	w._builder.HexDumpString(prefix, offsetFromStart, "SerializedContractKeyValueEntry.ContractName", string(w.ContractName))
	w._builder.HexDumpBytes(prefix, offsetFromStart, "SerializedContractKeyValueEntry.Key", w.Key)
	w._builder.HexDumpBytes(prefix, offsetFromStart, "SerializedContractKeyValueEntry.Value", w.Value)
	return nil
}

func (w *SerializedContractKeyValueEntryBuilder) GetSize() membuffers.Offset {
	if w == nil {
		return 0
	}
	return w._builder.GetSize()
}

func (w *SerializedContractKeyValueEntryBuilder) CalcRequiredSize() membuffers.Offset {
	if w == nil {
		return 0
	}
	w.Write(nil)
	return w._builder.GetSize()
}

func (w *SerializedContractKeyValueEntryBuilder) Build() *SerializedContractKeyValueEntry {
	buf := make([]byte, w.CalcRequiredSize())
	if w.Write(buf) != nil {
		return nil
	}
	return SerializedContractKeyValueEntryReader(buf)
}

func SerializedContractKeyValueEntryBuilderFromRaw(raw []byte) *SerializedContractKeyValueEntryBuilder {
	return &SerializedContractKeyValueEntryBuilder{_overrideWithRawBuffer: raw}
}

/////////////////////////////////////////////////////////////////////////////
// message SerializedMemoryPersistence

// reader

type SerializedMemoryPersistence struct {
	// BlockHeight primitives.BlockHeight
	// Timestamp primitives.TimestampNano
	// MerkleRootHash primitives.Sha256
	// Proposer primitives.NodeAddress
	// ReferenceTime primitives.TimestampSeconds
	// PreviousReferenceTime primitives.TimestampSeconds
	// Entries []SerializedContractKeyValueEntry

	// internal
	// implements membuffers.Message
	_message membuffers.InternalMessage
}

func (x *SerializedMemoryPersistence) String() string {
	if x == nil {
		return "<nil>"
	}
	return fmt.Sprintf("{BlockHeight:%s,Timestamp:%s,MerkleRootHash:%s,Proposer:%s,ReferenceTime:%s,PreviousReferenceTime:%s,Entries:%s,}", x.StringBlockHeight(), x.StringTimestamp(), x.StringMerkleRootHash(), x.StringProposer(), x.StringReferenceTime(), x.StringPreviousReferenceTime(), x.StringEntries())
}

var _SerializedMemoryPersistence_Scheme = []membuffers.FieldType{membuffers.TypeUint64, membuffers.TypeUint64, membuffers.TypeBytes, membuffers.TypeBytes, membuffers.TypeUint32, membuffers.TypeUint32, membuffers.TypeMessageArray}
var _SerializedMemoryPersistence_Unions = [][]membuffers.FieldType{}

func SerializedMemoryPersistenceReader(buf []byte) *SerializedMemoryPersistence {
	x := &SerializedMemoryPersistence{}
	x._message.Init(buf, membuffers.Offset(len(buf)), _SerializedMemoryPersistence_Scheme, _SerializedMemoryPersistence_Unions)
	return x
}

func (x *SerializedMemoryPersistence) IsValid() bool {
	return x._message.IsValid()
}

func (x *SerializedMemoryPersistence) Raw() []byte {
	return x._message.RawBuffer()
}

func (x *SerializedMemoryPersistence) Equal(y *SerializedMemoryPersistence) bool {
	if x == nil && y == nil {
		return true
	}
	if x == nil || y == nil {
		return false
	}
	return bytes.Equal(x.Raw(), y.Raw())
}

func (x *SerializedMemoryPersistence) BlockHeight() primitives.BlockHeight {
	return primitives.BlockHeight(x._message.GetUint64(0))
}

func (x *SerializedMemoryPersistence) RawBlockHeight() []byte {
	return x._message.RawBufferForField(0, 0)
}

func (x *SerializedMemoryPersistence) MutateBlockHeight(v primitives.BlockHeight) error {
	return x._message.SetUint64(0, uint64(v))
}

func (x *SerializedMemoryPersistence) StringBlockHeight() string {
	return fmt.Sprintf("%s", x.BlockHeight())
}

func (x *SerializedMemoryPersistence) Timestamp() primitives.TimestampNano {
	return primitives.TimestampNano(x._message.GetUint64(1))
}

func (x *SerializedMemoryPersistence) RawTimestamp() []byte {
	return x._message.RawBufferForField(1, 0)
}

func (x *SerializedMemoryPersistence) MutateTimestamp(v primitives.TimestampNano) error {
	return x._message.SetUint64(1, uint64(v))
}

func (x *SerializedMemoryPersistence) StringTimestamp() string {
	return fmt.Sprintf("%s", x.Timestamp())
}

func (x *SerializedMemoryPersistence) MerkleRootHash() primitives.Sha256 {
	return primitives.Sha256(x._message.GetBytes(2))
}

func (x *SerializedMemoryPersistence) RawMerkleRootHash() []byte {
	return x._message.RawBufferForField(2, 0)
}

func (x *SerializedMemoryPersistence) RawMerkleRootHashWithHeader() []byte {
	return x._message.RawBufferWithHeaderForField(2, 0)
}

func (x *SerializedMemoryPersistence) MutateMerkleRootHash(v primitives.Sha256) error {
	return x._message.SetBytes(2, []byte(v))
}

func (x *SerializedMemoryPersistence) StringMerkleRootHash() string {
	return fmt.Sprintf("%s", x.MerkleRootHash())
}

func (x *SerializedMemoryPersistence) Proposer() primitives.NodeAddress {
	return primitives.NodeAddress(x._message.GetBytes(3))
}

func (x *SerializedMemoryPersistence) RawProposer() []byte {
	return x._message.RawBufferForField(3, 0)
}

func (x *SerializedMemoryPersistence) RawProposerWithHeader() []byte {
	return x._message.RawBufferWithHeaderForField(3, 0)
}

func (x *SerializedMemoryPersistence) MutateProposer(v primitives.NodeAddress) error {
	return x._message.SetBytes(3, []byte(v))
}

func (x *SerializedMemoryPersistence) StringProposer() string {
	return fmt.Sprintf("%s", x.Proposer())
}

func (x *SerializedMemoryPersistence) ReferenceTime() primitives.TimestampSeconds {
	return primitives.TimestampSeconds(x._message.GetUint32(4))
}

func (x *SerializedMemoryPersistence) RawReferenceTime() []byte {
	return x._message.RawBufferForField(4, 0)
}

func (x *SerializedMemoryPersistence) MutateReferenceTime(v primitives.TimestampSeconds) error {
	return x._message.SetUint32(4, uint32(v))
}

func (x *SerializedMemoryPersistence) StringReferenceTime() string {
	return fmt.Sprintf("%s", x.ReferenceTime())
}

func (x *SerializedMemoryPersistence) PreviousReferenceTime() primitives.TimestampSeconds {
	return primitives.TimestampSeconds(x._message.GetUint32(5))
}

func (x *SerializedMemoryPersistence) RawPreviousReferenceTime() []byte {
	return x._message.RawBufferForField(5, 0)
}

func (x *SerializedMemoryPersistence) MutatePreviousReferenceTime(v primitives.TimestampSeconds) error {
	return x._message.SetUint32(5, uint32(v))
}

func (x *SerializedMemoryPersistence) StringPreviousReferenceTime() string {
	return fmt.Sprintf("%s", x.PreviousReferenceTime())
}

func (x *SerializedMemoryPersistence) EntriesIterator() *SerializedMemoryPersistenceEntriesIterator {
	return &SerializedMemoryPersistenceEntriesIterator{iterator: x._message.GetMessageArrayIterator(6)}
}

type SerializedMemoryPersistenceEntriesIterator struct {
	iterator *membuffers.Iterator
}

func (i *SerializedMemoryPersistenceEntriesIterator) HasNext() bool {
	return i.iterator.HasNext()
}

func (i *SerializedMemoryPersistenceEntriesIterator) NextEntries() *SerializedContractKeyValueEntry {
	b, s := i.iterator.NextMessage()
	return SerializedContractKeyValueEntryReader(b[:s])
}

func (x *SerializedMemoryPersistence) RawEntriesArray() []byte {
	return x._message.RawBufferForField(6, 0)
}

func (x *SerializedMemoryPersistence) RawEntriesArrayWithHeader() []byte {
	return x._message.RawBufferWithHeaderForField(6, 0)
}

func (x *SerializedMemoryPersistence) StringEntries() (res string) {
	res = "["
	for i := x.EntriesIterator(); i.HasNext(); {
		res += i.NextEntries().String() + ","
	}
	res += "]"
	return
}

// builder

type SerializedMemoryPersistenceBuilder struct {
	BlockHeight           primitives.BlockHeight
	Timestamp             primitives.TimestampNano
	MerkleRootHash        primitives.Sha256
	Proposer              primitives.NodeAddress
	ReferenceTime         primitives.TimestampSeconds
	PreviousReferenceTime primitives.TimestampSeconds
	Entries               []*SerializedContractKeyValueEntryBuilder

	// internal
	// implements membuffers.Builder
	_builder               membuffers.InternalBuilder
	_overrideWithRawBuffer []byte
}

func (w *SerializedMemoryPersistenceBuilder) arrayOfEntries() []membuffers.MessageWriter {
	res := make([]membuffers.MessageWriter, len(w.Entries))
	for i, v := range w.Entries {
		res[i] = v
	}
	return res
}

func (w *SerializedMemoryPersistenceBuilder) Write(buf []byte) (err error) {
	if w == nil {
		return
	}
	w._builder.NotifyBuildStart()
	defer w._builder.NotifyBuildEnd()
	defer func() {
		if r := recover(); r != nil {
			err = &membuffers.ErrBufferOverrun{}
		}
	}()
	if w._overrideWithRawBuffer != nil {
		return w._builder.WriteOverrideWithRawBuffer(buf, w._overrideWithRawBuffer)
	}
	w._builder.Reset()
	w._builder.WriteUint64(buf, uint64(w.BlockHeight))
	w._builder.WriteUint64(buf, uint64(w.Timestamp))
	w._builder.WriteBytes(buf, []byte(w.MerkleRootHash))
	w._builder.WriteBytes(buf, []byte(w.Proposer))
	w._builder.WriteUint32(buf, uint32(w.ReferenceTime))
	w._builder.WriteUint32(buf, uint32(w.PreviousReferenceTime))
	err = w._builder.WriteMessageArray(buf, w.arrayOfEntries())
	if err != nil {
		return
	}
	return nil
}

func (w *SerializedMemoryPersistenceBuilder) HexDump(prefix string, offsetFromStart membuffers.Offset) (err error) {
	if w == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			err = &membuffers.ErrBufferOverrun{}
		}
	}()
	w._builder.Reset()
	w._builder.HexDumpUint64(prefix, offsetFromStart, "SerializedMemoryPersistence.BlockHeight", uint64(w.BlockHeight))
	w._builder.HexDumpUint64(prefix, offsetFromStart, "SerializedMemoryPersistence.Timestamp", uint64(w.Timestamp))
	w._builder.HexDumpBytes(prefix, offsetFromStart, "SerializedMemoryPersistence.MerkleRootHash", []byte(w.MerkleRootHash))
	w._builder.HexDumpBytes(prefix, offsetFromStart, "SerializedMemoryPersistence.Proposer", []byte(w.Proposer))
	w._builder.HexDumpUint32(prefix, offsetFromStart, "SerializedMemoryPersistence.ReferenceTime", uint32(w.ReferenceTime))
	w._builder.HexDumpUint32(prefix, offsetFromStart, "SerializedMemoryPersistence.PreviousReferenceTime", uint32(w.PreviousReferenceTime))
	err = w._builder.HexDumpMessageArray(prefix, offsetFromStart, "SerializedMemoryPersistence.Entries", w.arrayOfEntries())
	if err != nil {
		return
	}
	return nil
}

func (w *SerializedMemoryPersistenceBuilder) GetSize() membuffers.Offset {
	if w == nil {
		return 0
	}
	return w._builder.GetSize()
}

func (w *SerializedMemoryPersistenceBuilder) CalcRequiredSize() membuffers.Offset {
	if w == nil {
		return 0
	}
	w.Write(nil)
	return w._builder.GetSize()
}

func (w *SerializedMemoryPersistenceBuilder) Build() *SerializedMemoryPersistence {
	buf := make([]byte, w.CalcRequiredSize())
	if w.Write(buf) != nil {
		return nil
	}
	return SerializedMemoryPersistenceReader(buf)
}

func SerializedMemoryPersistenceBuilderFromRaw(raw []byte) *SerializedMemoryPersistenceBuilder {
	return &SerializedMemoryPersistenceBuilder{_overrideWithRawBuffer: raw}
}
