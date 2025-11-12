package history

import (
	"errors"
	"pixerver/store"
)

const (
	HistoryDbPath = "history:" // interpreted as key prefix
)

var (
	HistoryBase  *store.Store
	SuccessStore *store.Store
	FailureStore *store.Store
)

// CreateDB opens the history stores (base/success/failure).
func CreateDB() (*store.Store, error) {
	var err error
	HistoryBase, err = store.New(HistoryDbPath)
	if err != nil {
		return nil, err
	}
	SuccessStore, err = store.New(HistoryDbPath + "success:")
	if err != nil {
		return nil, err
	}
	FailureStore, err = store.New(HistoryDbPath + "failure:")
	if err != nil {
		return nil, err
	}
	return HistoryBase, nil
}

// CloseDB closes all history stores.
func CloseDB() error {
	_ = SuccessStore.Close()
	_ = FailureStore.Close()
	return HistoryBase.Close()
}

// AddEntry stores a raw key/value pair in the history store.
func AddEntry(key, value []byte) error {
	return HistoryBase.Set(key, value)
}

// GetEntry retrieves a raw value by key from the history store.
func GetEntry(key []byte) ([]byte, error) {
	return HistoryBase.Get(key)
}

// DelEntry deletes an entry by key.
func DelEntry(key []byte) error {
	return HistoryBase.Del(key)
}

// AddSuccess stores a success entry under the success store.
func AddSuccess(key, value []byte) error {
	return SuccessStore.Set(key, value)
}

// AddFailure stores a failure entry under the failure store.
func AddFailure(key, value []byte) error {
	return FailureStore.Set(key, value)
}

// GetSuccess retrieves a success entry by key (non-prefixed key expected).
func GetSuccess(key []byte) ([]byte, error) {
	return SuccessStore.Get(key)
}

// GetFailure retrieves a failure entry by key (non-prefixed key expected).
func GetFailure(key []byte) ([]byte, error) {
	return FailureStore.Get(key)
}

// DelSuccess deletes a success entry by key (non-prefixed key expected).
func DelSuccess(key []byte) error {
	return SuccessStore.Del(key)
}

// DelFailure deletes a failure entry by key (non-prefixed key expected).
func DelFailure(key []byte) error {
	return FailureStore.Del(key)
}

// HistoryKV represents a stored history item with key and value.
type HistoryKV struct {
	Key   []byte
	Value []byte
}

// ListHistory lists entries for either success or failure stores.
func ListHistory(which string) ([]HistoryKV, error) {
	if which == "success" {
		if SuccessStore == nil {
			return nil, errors.New("history not open")
		}
		kvs, err := SuccessStore.List()
		if err != nil {
			return nil, err
		}
		out := make([]HistoryKV, 0, len(kvs))
		for _, k := range kvs {
			out = append(out, HistoryKV{Key: k.Key, Value: k.Value})
		}
		return out, nil
	}
	if which == "failure" {
		if FailureStore == nil {
			return nil, errors.New("history not open")
		}
		kvs, err := FailureStore.List()
		if err != nil {
			return nil, err
		}
		out := make([]HistoryKV, 0, len(kvs))
		for _, k := range kvs {
			out = append(out, HistoryKV{Key: k.Key, Value: k.Value})
		}
		return out, nil
	}
	return nil, errors.New("unknown history type")
}

// ListSuccesses returns all success entries.
func ListSuccesses() ([]HistoryKV, error) {
	return ListHistory("success")
}

// ListFailures returns all failure entries.
func ListFailures() ([]HistoryKV, error) {
	return ListHistory("failure")
}
