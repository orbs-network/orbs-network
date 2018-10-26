package adapter

import (
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReadStateWithNonExistingContractName(t *testing.T) {
	d := NewInMemoryStatePersistence()
	_, _, err := d.Read("foo", "")
	require.NoError(t, err, "unexpected error")
}

func TestWriteStateAddAndRemoveKeyFromPersistentStorage(t *testing.T) {
	d := newDriver()

	d.writeSingleValueBlock(1, "foo", "foo", "bar")

	record, ok, err := d.Read("foo", "foo")
	require.NoError(t, err, "unexpected error")
	require.EqualValues(t, true, ok, "after writing a key it should exist")
	require.EqualValues(t, "foo", record.Key(), "after writing a key/value it should be returned")
	require.EqualValues(t, "bar", record.Value(), "after writing a key/value it should be returned")

	d.writeSingleValueBlock(1, "foo", "foo", "")

	_, ok, err = d.Read("foo", "foo")
	require.NoError(t, err, "unexpected error")
	require.EqualValues(t, false, ok, "writing zero value to state did not remove key")
}

func TestEach(t *testing.T) {
	d := newDriver()

	d.writeSingleValueBlock(1, "foo", "foo", "bar")
	d.writeSingleValueBlock(2, "foo", "baz", "qux")

	values := map[string]string{}
	d.Each(func(c primitives.ContractName, r *protocol.StateRecord){
		values[string(r.Key())] = string(r.Value())
	})
	require.EqualValues(t, map[string]string{"foo": "bar", "baz": "qux"}, values)

	d.writeSingleValueBlock(3, "foo", "baz", "")

	values = map[string]string{}
	d.Each(func(c primitives.ContractName, r *protocol.StateRecord){
		values[string(r.Key())] = string(r.Value())
	})
	require.EqualValues(t, map[string]string{"foo": "bar"}, values)
}

type driver struct {
	*InMemoryStatePersistence
}

func newDriver() *driver {
	return &driver{
		NewInMemoryStatePersistence(),
	}
}

func (d *driver) writeSingleValueBlock(h primitives.BlockHeight, c, k, v string) error {
	record := (&protocol.StateRecordBuilder{Key: []byte(k), Value: []byte(v)}).Build()
	diff := ChainState{primitives.ContractName(c): {k: record}}
	return d.InMemoryStatePersistence.Write(h, 0, []byte{}, diff)
}
