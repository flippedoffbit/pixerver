package tasks

import (
	"errors"
	"pixerver/store"
)

const (
	TaskDbPath = "tasks:" // now interpreted as key prefix for store
)

var TaskDB *store.Store

// CreateDB creates and opens the Store for tasks.
func CreateDB() (*store.Store, error) {
	var err error
	TaskDB, err = store.New(TaskDbPath)
	return TaskDB, err
}

// CloseDB closes the Store client.
func CloseDB() error {
	return TaskDB.Close()
}

// AddTask adds a task to the Store.
func AddTask(key, value []byte) error {
	return TaskDB.Set(key, value)
}

// GetTask retrieves a task from the Store.
func GetTask(key []byte) ([]byte, error) {
	return TaskDB.Get(key)
}

// DelTask deletes a task from the Store.
func DelTask(key []byte) error {
	return TaskDB.Del(key)
}

// TaskKV represents a key/value entry for a task stored in Store.
type TaskKV struct {
	Key   []byte
	Value []byte
}

// ListTasks returns all task key/value pairs stored in the TaskDB.
func ListTasks() ([]TaskKV, error) {
	if TaskDB == nil {
		return nil, errors.New("task db not open")
	}
	kvs, err := TaskDB.List()
	if err != nil {
		return nil, err
	}
	out := make([]TaskKV, 0, len(kvs))
	for _, k := range kvs {
		out = append(out, TaskKV{Key: k.Key, Value: k.Value})
	}
	return out, nil
}
