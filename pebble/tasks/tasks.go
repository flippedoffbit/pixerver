package tasks

import (
	"errors"
	pebblecore "pixerver/pebble/core"

	"github.com/cockroachdb/pebble"
)

const (
	TaskDbPath = "data/pebble.db"
)

var TaskDB *pebble.DB

// CreateDB creates and opens a Pebble DB at the specified path.
func CreateDB() (*pebble.DB, error) {
	var err error
	TaskDB, err = pebblecore.Open(TaskDbPath)
	return TaskDB, err
}

// CloseDB closes the Pebble DB.
func CloseDB() error {
	return pebblecore.Close(TaskDB)
}

// AddTask adds a task to the Pebble DB.

func AddTask(key, value []byte) error {
	return pebblecore.AddEntry(TaskDB, key, value)
}

// GetTask retrieves a task from the Pebble DB.
func GetTask(key []byte) ([]byte, error) {
	return pebblecore.GetEntry(TaskDB, key)
}

// DelTask deletes a task from the Pebble DB.
func DelTask(key []byte) error {
	return pebblecore.DelEntry(TaskDB, key)
}

// TaskKV represents a key/value entry for a task stored in Pebble.
type TaskKV struct {
	Key   []byte
	Value []byte
}

// ListTasks returns all task key/value pairs stored in the TaskDB.
// Caller receives copies of keys and values and can mutate them safely.
func ListTasks() ([]TaskKV, error) {
	if TaskDB == nil {
		return nil, errors.New("task db not open")
	}

	it, err := TaskDB.NewIter(nil)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var out []TaskKV
	for ok := it.First(); ok; ok = it.Next() {
		// copy key and value so callers don't hold references to memmapped data
		k := append([]byte(nil), it.Key()...)
		v := append([]byte(nil), it.Value()...)
		out = append(out, TaskKV{Key: k, Value: v})
	}

	if err := it.Error(); err != nil {
		return nil, err
	}
	return out, nil
}
