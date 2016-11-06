package mqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

type RedisMQ struct {
	pool      *redis.Pool
	queueName string
	ticker    *time.Ticker
	prefix    string
}

func NewRedisMQ(url *url.URL) (*RedisMQ, error) {

	pool := &redis.Pool{
		MaxIdle: 4,
		// I'm not sure if allowing the pool to block if more than 16 connections are required is a good idea.
		MaxActive:   16,
		Wait:        true,
		IdleTimeout: 300 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.DialURL(url.String())
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	// Force a connection so we can fail in case of error.
	conn := pool.Get()
	if err := conn.Err(); err != nil {
		logrus.WithError(err).Fatal("Error connecting to redis")
	}
	conn.Close()

	mq := &RedisMQ{
		pool:   pool,
		ticker: time.NewTicker(time.Second),
		prefix: url.Path,
	}
	mq.queueName = mq.k("queue")
	logrus.WithFields(logrus.Fields{"name": mq.queueName}).Info("Redis initialized with queue name")

	mq.start()
	return mq, nil
}

func (mq *RedisMQ) k(s string) string {
	return mq.prefix + s
}

func getFirstKeyValue(resp map[string]string) (string, string, error) {

	for key, value := range resp {
		return key, value, nil
	}
	return "", "", errors.New("Blank map")
}

func (mq *RedisMQ) processPendingReservations(conn redis.Conn) {
	resp, err := redis.StringMap(conn.Do("ZRANGE", mq.k("timeouts"), 0, 0, "WITHSCORES"))
	if mq.checkNilResponse(err) || len(resp) == 0 {
		return
	}
	if err != nil {
		logrus.WithError(err).Error("Redis command error")
	}

	reservationId, timeoutString, err := getFirstKeyValue(resp)
	if err != nil {
		logrus.WithError(err).Error("error getting kv")
		return
	}

	timeout, err := strconv.ParseInt(timeoutString, 10, 64)
	if err != nil || timeout > time.Now().Unix() {
		return
	}
	response, err := redis.Bytes(conn.Do("HGET", mq.k("timeout_jobs"), reservationId))
	if mq.checkNilResponse(err) {
		return
	}
	if err != nil {
		logrus.WithError(err).Error("redis get timeout_jobs error")
		return
	}

	var job models.Task
	err = json.Unmarshal(response, &job)
	if err != nil {
		logrus.WithError(err).Error("error unmarshaling job json")
		return
	}

	conn.Do("ZREM", mq.k("timeouts"), reservationId)
	conn.Do("HDEL", mq.k("timeout_jobs"), reservationId)
	conn.Do("HDEL", mq.k("reservations"), job.ID)
	redisPush(conn, mq.queueName, &job)
}

func (mq *RedisMQ) processDelayedTasks(conn redis.Conn) {
	// List of reservation ids between -inf time and the current time will get us
	// everything that is now ready to be queued.
	now := time.Now().UTC().Unix()
	resIds, err := redis.Strings(conn.Do("ZRANGEBYSCORE", mq.k("delays"), "-inf", now))
	if err != nil {
		logrus.WithError(err).Error("Error getting delayed jobs")
		return
	}

	for _, resId := range resIds {
		// Might be a good idea to do this transactionally so we do not have left over reservationIds if the delete fails.
		buf, err := redis.Bytes(conn.Do("HGET", mq.k("delayed_jobs"), resId))
		// If:
		// a) A HSET in Push() failed, or
		// b) A previous zremrangebyscore failed,
		// we can get ids that we never associated with a job, or already placed in the queue, just skip these.
		if err == redis.ErrNil {
			continue
		} else if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"reservationId": resId}).Error("Error HGET delayed_jobs")
			continue
		}

		var job models.Task
		err = json.Unmarshal(buf, &job)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"buf": buf, "reservationId": resId}).Error("Error unmarshaling job")
			return
		}

		_, err = redisPush(conn, mq.queueName, &job)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"reservationId": resId}).Error("Pushing delayed job")
			return
		}
		conn.Do("HDEL", mq.k("delayed_jobs"), resId)
	}

	// Remove everything we processed.
	conn.Do("ZREMRANGEBYSCORE", mq.k("delays"), "-inf", now)
}

func (mq *RedisMQ) start() {
	go func() {
		conn := mq.pool.Get()
		defer conn.Close()
		if err := conn.Err(); err != nil {
			logrus.WithError(err).Fatal("Could not start redis MQ reservation system")
		}

		for range mq.ticker.C {
			mq.processPendingReservations(conn)
			mq.processDelayedTasks(conn)
		}
	}()
}

func redisPush(conn redis.Conn, queue string, job *models.Task) (*models.Task, error) {
	buf, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	_, err = conn.Do("LPUSH", fmt.Sprintf("%s%d", queue, *job.Priority), buf)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (mq *RedisMQ) delayTask(conn redis.Conn, job *models.Task) (*models.Task, error) {
	buf, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}

	resp, err := redis.Int64(conn.Do("INCR", mq.k("delays_counter")))
	if err != nil {
		return nil, err
	}

	reservationId := strconv.FormatInt(resp, 10)

	// Timestamp -> resID
	_, err = conn.Do("ZADD", mq.k("delays"), time.Now().UTC().Add(time.Duration(job.Delay)*time.Second).Unix(), reservationId)
	if err != nil {
		return nil, err
	}

	// resID -> Task
	_, err = conn.Do("HSET", mq.k("delayed_jobs"), reservationId, buf)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (mq *RedisMQ) Push(ctx context.Context, job *models.Task) (*models.Task, error) {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	defer log.Println("Pushed to MQ")

	conn := mq.pool.Get()
	defer conn.Close()

	if job.Delay > 0 {
		return mq.delayTask(conn, job)
	}
	return redisPush(conn, mq.queueName, job)
}
func (mq *RedisMQ) checkNilResponse(err error) bool {
	return err != nil && err.Error() == redis.ErrNil.Error()
}

// Would be nice to switch to this model http://redis.io/commands/rpoplpush#pattern-reliable-queue
func (mq *RedisMQ) Reserve(ctx context.Context) (*models.Task, error) {

	conn := mq.pool.Get()
	defer conn.Close()
	var job models.Task
	var resp []byte
	var err error
	for i := 2; i >= 0; i-- {
		resp, err = redis.Bytes(conn.Do("RPOP", fmt.Sprintf("%s%d", mq.queueName, i)))
		if mq.checkNilResponse(err) {
			if i == 0 {
				// Out of queues!
				return nil, nil
			}

			// No valid job on this queue, try lower priority queue.
			continue
		} else if err != nil {
			// Some other error!
			return nil, err
		}

		// We got a valid high priority job.
		break
	}

	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(resp, &job)
	if err != nil {
		return nil, err
	}

	response, err := redis.Int64(conn.Do("INCR", mq.queueName+"_incr"))
	if err != nil {
		return nil, err
	}
	reservationId := strconv.FormatInt(response, 10)
	_, err = conn.Do("ZADD", "timeout:", time.Now().Add(time.Minute).Unix(), reservationId)
	if err != nil {
		return nil, err
	}
	_, err = conn.Do("HSET", "timeout", reservationId, resp)
	if err != nil {
		return nil, err
	}

	// Map from job.ID -> reservation ID
	_, err = conn.Do("HSET", "reservations", job.ID, reservationId)
	if err != nil {
		return nil, err
	}

	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	log.Println("Reserved")

	return &job, nil
}

func (mq *RedisMQ) Delete(ctx context.Context, job *models.Task) error {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	defer log.Println("Deleted")

	conn := mq.pool.Get()
	defer conn.Close()
	resId, err := conn.Do("HGET", "reservations", job.ID)
	if err != nil {
		return err
	}
	_, err = conn.Do("HDEL", "reservations", job.ID)
	if err != nil {
		return err
	}
	_, err = conn.Do("ZREM", "timeout:", resId)
	if err != nil {
		return err
	}
	_, err = conn.Do("HDEL", "timeout", resId)
	return err
}
