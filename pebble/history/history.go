package history

import (
	"bytes"
	"errors"
	pebblecore "pixerver/pebble/core"

	"github.com/cockroachdb/pebble"
)

const (
	HistoryDbPath = "data/history.db"
	SuccessPrefix = "success:"
	FailurePrefix = "failure:"
)

var HistoryDB *pebble.DB

// CreateDB opens (or creates) the history DB.
func CreateDB() (*pebble.DB, error) {
	var err error
	HistoryDB, err = pebblecore.Open(HistoryDbPath)
	return HistoryDB, err
}

// CloseDB closes the history DB.
func CloseDB() error {
	return pebblecore.Close(HistoryDB)
}

// AddEntry stores a raw key/value pair in the history DB.
func AddEntry(key, value []byte) error {
	return pebblecore.AddEntry(HistoryDB, key, value)
}

// GetEntry retrieves a raw value by key from the history DB.
func GetEntry(key []byte) ([]byte, error) {
	return pebblecore.GetEntry(HistoryDB, key)
}

// DelEntry deletes an entry by key.
func DelEntry(key []byte) error {
	return pebblecore.DelEntry(HistoryDB, key)
}

// AddSuccess stores a success entry under the success prefix.
func AddSuccess(key, value []byte) error {
	pref := append([]byte(SuccessPrefix), key...)
	return AddEntry(pref, value)
}

// AddFailure stores a failure entry under the failure prefix.
func AddFailure(key, value []byte) error {
	pref := append([]byte(FailurePrefix), key...)
	return AddEntry(pref, value)
}

// GetSuccess retrieves a success entry by key (non-prefixed key expected).
func GetSuccess(key []byte) ([]byte, error) {
	pref := append([]byte(SuccessPrefix), key...)
	return GetEntry(pref)
}

// GetFailure retrieves a failure entry by key (non-prefixed key expected).
func GetFailure(key []byte) ([]byte, error) {
	pref := append([]byte(FailurePrefix), key...)
	return GetEntry(pref)
}

// DelSuccess deletes a success entry by key (non-prefixed key expected).
func DelSuccess(key []byte) error {
	pref := append([]byte(SuccessPrefix), key...)
	return DelEntry(pref)
}

// DelFailure deletes a failure entry by key (non-prefixed key expected).
func DelFailure(key []byte) error {
	pref := append([]byte(FailurePrefix), key...)
	return DelEntry(pref)
}

// HistoryKV represents a stored history item with key and value.
type HistoryKV struct {
	Key   []byte
	Value []byte
}

// ListHistory lists entries under the given prefix ("success:" or "failure:").
// Returned keys have the prefix stripped.
func ListHistory(prefix string) ([]HistoryKV, error) {
	if HistoryDB == nil {
		return nil, errors.New("history db not open")
	}

	it, err := HistoryDB.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var out []HistoryKV
	p := []byte(prefix)
	for ok := it.First(); ok; ok = it.Next() {
		k := it.Key()
		if !bytes.HasPrefix(k, p) {
			continue
		}
		// strip prefix
		stripped := append([]byte(nil), k[len(p):]...)
		v := append([]byte(nil), it.Value()...)
		out = append(out, HistoryKV{Key: stripped, Value: v})
	}

	if err := it.Error(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListSuccesses returns all success entries.
func ListSuccesses() ([]HistoryKV, error) {
	return ListHistory(SuccessPrefix)
}

// ListFailures returns all failure entries.
func ListFailures() ([]HistoryKV, error) {
	return ListHistory(FailurePrefix)
}
