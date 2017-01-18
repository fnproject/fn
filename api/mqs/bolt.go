package mqs

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

type BoltDbMQ struct {
	db     *bolt.DB
	ticker *time.Ticker
}

type BoltDbConfig struct {
	FileName string `mapstructure:"filename"`
}

func jobKey(jobID string) []byte {
	b := make([]byte, len(jobID)+1)
	b[0] = 'j'
	copy(b[1:], []byte(jobID))
	return b
}

const timeoutToIDKeyPrefix = "id:"

func timeoutToIDKey(timeout []byte) []byte {
	b := make([]byte, len(timeout)+len(timeoutToIDKeyPrefix))
	copy(b[:], []byte(timeoutToIDKeyPrefix))
	copy(b[len(timeoutToIDKeyPrefix):], []byte(timeout))
	return b
}

var delayQueueName = []byte("functions_delay")

func queueName(i int) []byte {
	return []byte(fmt.Sprintf("functions_%d_queue", i))
}

func timeoutName(i int) []byte {
	return []byte(fmt.Sprintf("functions_%d_timeout", i))
}

func NewBoltMQ(url *url.URL) (*BoltDbMQ, error) {
	dir := filepath.Dir(url.Path)
	log := logrus.WithFields(logrus.Fields{"mq": url.Scheme, "dir": dir})
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.WithError(err).Errorln("Could not create data directory for mq")
		return nil, err
	}
	db, err := bolt.Open(url.Path, 0655, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.WithError(err).Errorln("Could not open BoltDB file for MQ")
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for i := 0; i < 3; i++ {
			_, err := tx.CreateBucketIfNotExists(queueName(i))
			if err != nil {
				log.WithError(err).Errorln("Error creating bucket")
				return err
			}
			_, err = tx.CreateBucketIfNotExists(timeoutName(i))
			if err != nil {
				log.WithError(err).Errorln("Error creating timeout bucket")
				return err
			}
		}
		_, err = tx.CreateBucketIfNotExists(delayQueueName)
		if err != nil {
			log.WithError(err).Errorln("Error creating delay bucket")
			return err
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorln("Error creating timeout bucket")
		return nil, err
	}

	ticker := time.NewTicker(time.Second)
	mq := &BoltDbMQ{
		ticker: ticker,
		db:     db,
	}
	mq.Start()
	log.WithFields(logrus.Fields{"file": url.Path}).Debug("BoltDb initialized")
	return mq, nil
}

func (mq *BoltDbMQ) Start() {
	go func() {
		// It would be nice to switch to a tick-less, next-event Timer based model.
		for range mq.ticker.C {
			err := mq.db.Update(func(tx *bolt.Tx) error {
				now := uint64(time.Now().UnixNano())
				for i := 0; i < 3; i++ {
					// Assume our timeouts bucket exists and has resKey encoded keys.
					jobBucket := tx.Bucket(queueName(i))
					timeoutBucket := tx.Bucket(timeoutName(i))
					c := timeoutBucket.Cursor()

					var err error
					for k, v := c.Seek([]byte(resKeyPrefix)); k != nil; k, v = c.Next() {
						reserved, id := resKeyToProperties(k)
						if reserved > now {
							break
						}
						err = jobBucket.Put(id, v)
						if err != nil {
							return err
						}
						timeoutBucket.Delete(k)
						timeoutBucket.Delete(timeoutToIDKey(k))
					}
				}

				return nil
			})
			if err != nil {
				logrus.WithError(err).Error("boltdb reservation check error")
			}

			err = mq.db.Update(func(tx *bolt.Tx) error {
				now := uint64(time.Now().UnixNano())
				// Assume our timeouts bucket exists and has resKey encoded keys.
				delayBucket := tx.Bucket(delayQueueName)
				c := delayBucket.Cursor()

				var err error
				for k, v := c.Seek([]byte(resKeyPrefix)); k != nil; k, v = c.Next() {
					reserved, id := resKeyToProperties(k)
					if reserved > now {
						break
					}

					priority := binary.BigEndian.Uint32(v)
					job := delayBucket.Get(id)
					if job == nil {
						// oops
						logrus.Warnf("Expected delayed job, none found with id %s", id)
						continue
					}

					jobBucket := tx.Bucket(queueName(int(priority)))
					err = jobBucket.Put(id, job)
					if err != nil {
						return err
					}

					err := delayBucket.Delete(k)
					if err != nil {
						return err
					}

					return delayBucket.Delete(id)
				}
				return nil
			})
			if err != nil {
				logrus.WithError(err).Error("boltdb delay check error")
			}
		}
	}()
}

// We insert a "reservation" at readyAt, and store the json blob at the msg
// key. The timer loop plucks this out and puts it in the jobs bucket when the
// time elapses. The value stored at the reservation key is the priority.
func (mq *BoltDbMQ) delayTask(job *models.Task) (*models.Task, error) {
	readyAt := time.Now().Add(time.Duration(job.Delay) * time.Second)
	err := mq.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(delayQueueName)
		id, _ := b.NextSequence()
		buf, err := json.Marshal(job)
		if err != nil {
			return err
		}

		key := msgKey(id)
		err = b.Put(key, buf)
		if err != nil {
			return err
		}

		pb := make([]byte, 4)
		binary.BigEndian.PutUint32(pb[:], uint32(*job.Priority))
		reservation := resKey(key, readyAt)
		return b.Put(reservation, pb)
	})
	return job, err
}

func (mq *BoltDbMQ) Push(ctx context.Context, job *models.Task) (*models.Task, error) {
	ctx, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	log.Println("Pushed to MQ")

	if job.Delay > 0 {
		return mq.delayTask(job)
	}

	err := mq.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(queueName(int(*job.Priority)))

		id, _ := b.NextSequence()

		buf, err := json.Marshal(job)
		if err != nil {
			return err
		}

		return b.Put(msgKey(id), buf)
	})
	if err != nil {
		return nil, err
	}

	return job, nil

}

const msgKeyPrefix = "j:"
const msgKeyLength = len(msgKeyPrefix) + 8
const resKeyPrefix = "r:"

// r:<timestamp>:msgKey
// The msgKey is used to introduce uniqueness within the timestamp. It probably isn't required.
const resKeyLength = len(resKeyPrefix) + msgKeyLength + 8

func msgKey(v uint64) []byte {
	b := make([]byte, msgKeyLength)
	copy(b[:], []byte(msgKeyPrefix))
	binary.BigEndian.PutUint64(b[len(msgKeyPrefix):], v)
	return b
}

func resKey(jobKey []byte, reservedUntil time.Time) []byte {
	b := make([]byte, resKeyLength)
	copy(b[:], []byte(resKeyPrefix))
	binary.BigEndian.PutUint64(b[len(resKeyPrefix):], uint64(reservedUntil.UnixNano()))
	copy(b[len(resKeyPrefix)+8:], jobKey)
	return b
}

func resKeyToProperties(key []byte) (uint64, []byte) {
	if len(key) != resKeyLength {
		return 0, nil
	}

	reservedUntil := binary.BigEndian.Uint64(key[len(resKeyPrefix):])
	return reservedUntil, key[len(resKeyPrefix)+8:]
}

func (mq *BoltDbMQ) Reserve(ctx context.Context) (*models.Task, error) {
	// Start a writable transaction.
	tx, err := mq.db.Begin(true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for i := 2; i >= 0; i-- {
		// Use the transaction...
		b := tx.Bucket(queueName(i))
		c := b.Cursor()
		key, value := c.Seek([]byte(msgKeyPrefix))
		if key == nil {
			// No jobs, try next bucket
			continue
		}

		b.Delete(key)

		var job models.Task
		err = json.Unmarshal([]byte(value), &job)
		if err != nil {
			return nil, err
		}

		reservationKey := resKey(key, time.Now().Add(time.Minute))
		b = tx.Bucket(timeoutName(i))
		// Reserve introduces 3 keys in timeout bucket:
		// Save reservationKey -> Task to allow release
		// Save job.ID -> reservationKey to allow Deletes
		// Save reservationKey -> job.ID to allow clearing job.ID -> reservationKey in recovery without unmarshaling the job.
		// On Delete:
		// We have job ID, we get the reservationKey
		// Delete job.ID -> reservationKey
		// Delete reservationKey -> job.ID
		// Delete reservationKey -> Task
		// On Release:
		// We have reservationKey, we get the jobID
		// Delete reservationKey -> job.ID
		// Delete job.ID -> reservationKey
		// Move reservationKey -> Task to job bucket.
		b.Put(reservationKey, value)
		b.Put(jobKey(job.ID), reservationKey)
		b.Put(timeoutToIDKey(reservationKey), []byte(job.ID))

		// Commit the transaction and check for error.
		if err := tx.Commit(); err != nil {
			return nil, err
		}

		_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
		log.Println("Reserved")

		return &job, nil
	}

	return nil, nil
}

func (mq *BoltDbMQ) Delete(ctx context.Context, job *models.Task) error {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	defer log.Println("Deleted")

	return mq.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(timeoutName(int(*job.Priority)))
		k := jobKey(job.ID)

		reservationKey := b.Get(k)
		if reservationKey == nil {
			return errors.New("Not found")
		}

		for _, k := range [][]byte{k, timeoutToIDKey(reservationKey), reservationKey} {
			err := b.Delete(k)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
