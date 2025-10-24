package pebblecore

import (
	"pixerver/logger"

	"github.com/cockroachdb/pebble"
)

// Open opens (or creates) a Pebble DB at the provided path and returns the
// *pebble.DB. Caller must call Close when finished.
func Open(path string) (*pebble.DB, error) {
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		logger.Errorf("failed to open database %s: %v", path, err)
		return nil, err
	}
	logger.Infof("opened pebble db at %s", path)
	return db, nil
}

// Close closes the database and logs any error returned.
func Close(db *pebble.DB) error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		logger.Errorf("error closing db: %v", err)
		return err
	}
	logger.Info("db closed")
	return nil
}

// AddEntry sets a key/value pair in the DB.
func AddEntry(db *pebble.DB, key, value []byte) error {
	if db == nil {
		return pebble.ErrClosed
	}
	if err := db.Set(key, value, pebble.NoSync); err != nil {
		logger.Errorf("set failed: %v", err)
		return err
	}
	logger.Debugf("set key=%x", key)
	return nil
}

// GetEntry retrieves a value for the provided key.
func GetEntry(db *pebble.DB, key []byte) ([]byte, error) {
	if db == nil {
		return nil, pebble.ErrClosed
	}
	v, closer, err := db.Get(key)
	if err != nil {
		logger.Warnf("get key=%x: %v", key, err)
		return nil, err
	}
	// pebble Get returns a slice referencing the DB. Make a copy to be safe.
	val := append([]byte(nil), v...)
	if closer != nil {
		_ = closer.Close()
	}
	logger.Debugf("got key=%x len=%d", key, len(val))
	return val, nil
}

// DelEntry deletes a key from the DB.
func DelEntry(db *pebble.DB, key []byte) error {
	if db == nil {
		return pebble.ErrClosed
	}
	if err := db.Delete(key, pebble.NoSync); err != nil {
		logger.Errorf("delete key=%x: %v", key, err)
		return err
	}
	logger.Debugf("deleted key=%x", key)
	return nil
}
