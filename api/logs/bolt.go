package logs

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"context"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

type BoltLogDatastore struct {
	callLogsBucket []byte
	db             *bolt.DB
	log            logrus.FieldLogger
	datastore      models.Datastore
}

func NewBolt(url *url.URL) (models.FnLog, error) {
	dir := filepath.Dir(url.Path)
	log := logrus.WithFields(logrus.Fields{"logdb": url.Scheme, "dir": dir})
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.WithError(err).Errorln("Could not create data directory for log.db")
		return nil, err
	}
	log.WithFields(logrus.Fields{"path": url.Path}).Debug("Creating bolt log.db")
	db, err := bolt.Open(url.Path, 0655, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.WithError(err).Errorln("Error on bolt.Open")
		return nil, err
	}
	// I don't think we need a prefix here do we? Made it blank. If we do, we should call the query param "prefix" instead of bucket.
	bucketPrefix := ""
	if url.Query()["bucket"] != nil {
		bucketPrefix = url.Query()["bucket"][0]
	}
	callLogsBucketName := []byte(bucketPrefix + "call_logs")
	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{callLogsBucketName} {
			_, err := tx.CreateBucketIfNotExists(name)
			if err != nil {
				log.WithError(err).WithFields(logrus.Fields{"name": name}).Error("create bucket")
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorln("Error creating bolt buckets")
		return nil, err
	}

	fnl := &BoltLogDatastore{
		callLogsBucket: callLogsBucketName,
		db:             db,
		log:            log,
	}
	log.WithFields(logrus.Fields{"prefix": bucketPrefix, "file": url.Path}).Debug("BoltDB initialized")

	return NewValidator(fnl), nil
}

func (fnl *BoltLogDatastore) InsertLog(ctx context.Context, callID string, callLog string) error {
	log := &models.FnCallLog{
		CallID: callID,
		Log:    callLog,
	}
	id := []byte(callID)
	err := fnl.db.Update(
		func(tx *bolt.Tx) error {
			bIm := tx.Bucket(fnl.callLogsBucket)
			buf, err := json.Marshal(log)
			if err != nil {
				return err
			}
			err = bIm.Put(id, buf)
			if err != nil {
				return err
			}
			return nil
		})

	return err
}

func (fnl *BoltLogDatastore) GetLog(ctx context.Context, callID string) (*models.FnCallLog, error) {
	var res *models.FnCallLog
	err := fnl.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(fnl.callLogsBucket)
		v := b.Get([]byte(callID))
		if v != nil {
			fnCall := &models.FnCallLog{}
			err := json.Unmarshal(v, fnCall)
			if err != nil {
				return nil
			}
			res = fnCall
		} else {
			return models.ErrCallLogNotFound
		}
		return nil
	})
	return res, err
}

func (fnl *BoltLogDatastore) DeleteLog(ctx context.Context, callID string) error {
	_, err := fnl.GetLog(ctx, callID)
	//means object does not exist
	if err != nil {
		return nil
	}

	id := []byte(callID)
	err = fnl.db.Update(func(tx *bolt.Tx) error {
		bIm := tx.Bucket(fnl.callLogsBucket)
		err := bIm.Delete(id)
		return err
	})
	return err
}
