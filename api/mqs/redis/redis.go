package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/garyburd/redigo/redis"
	"github.com/sirupsen/logrus"
)

type RedisMQ struct {
	pool      *redis.Pool
	queueName string
	ticker    *time.Ticker
	prefix    string
}

type redisProvider int

func (redisProvider) Supports(url *url.URL) bool {
	switch url.Scheme {
	case "redis":
		return true
	}
	return false
}

func (redisProvider) String() string {
	return "redis"
}

func (redisProvider) New(url *url.URL) (models.MessageQueue, error) {
	pool := &redis.Pool{
		MaxIdle: 512,
		// I'm not sure if allowing the pool to block if more than 16 connections are required is a good idea.
		MaxActive:   512,
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

func (mq *RedisMQ) processPendingReservations() {
	conn := mq.pool.Get()
	defer conn.Close()

	resp, err := redis.StringMap(conn.Do("ZRANGE", mq.k("timeouts"), 0, 0, "WITHSCORES"))
	if mq.checkNilResponse(err) || len(resp) == 0 {
		return
	}
	if err != nil {
		logrus.WithError(err).Error("Redis command error")
	}

	reservationID, timeoutString, err := getFirstKeyValue(resp)
	if err != nil {
		logrus.WithError(err).Error("error getting kv")
		return
	}

	timeout, err := strconv.ParseInt(timeoutString, 10, 64)
	if err != nil || timeout > time.Now().Unix() {
		return
	}
	response, err := redis.Bytes(conn.Do("HGET", mq.k("timeout_jobs"), reservationID))
	if mq.checkNilResponse(err) {
		return
	}
	if err != nil {
		logrus.WithError(err).Error("redis get timeout_jobs error")
		return
	}

	var job models.Call
	err = json.Unmarshal(response, &job)
	if err != nil {
		logrus.WithError(err).Error("error unmarshaling job json")
		return
	}

	// :( because fuck atomicity right?
	conn.Do("ZREM", mq.k("timeouts"), reservationID)
	conn.Do("HDEL", mq.k("timeout_jobs"), reservationID)
	conn.Do("HDEL", mq.k("reservations"), job.ID)
	redisPush(conn, mq.queueName, &job)
}

func (mq *RedisMQ) processDelayedCalls() {
	conn := mq.pool.Get()
	defer conn.Close()

	// List of reservation ids between -inf time and the current time will get us
	// everything that is now ready to be queued.
	now := time.Now().UTC().Unix()
	resIds, err := redis.Strings(conn.Do("ZRANGEBYSCORE", mq.k("delays"), "-inf", now))
	if err != nil {
		logrus.WithError(err).Error("Error getting delayed jobs")
		return
	}

	for _, resID := range resIds {
		// Might be a good idea to do this transactionally so we do not have left over reservationIds if the delete fails.
		buf, err := redis.Bytes(conn.Do("HGET", mq.k("delayed_jobs"), resID))
		// If:
		// a) A HSET in Push() failed, or
		// b) A previous zremrangebyscore failed,
		// we can get ids that we never associated with a job, or already placed in the queue, just skip these.
		if err == redis.ErrNil {
			continue
		} else if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"reservation_id": resID}).Error("Error HGET delayed_jobs")
			continue
		}

		var job models.Call
		err = json.Unmarshal(buf, &job)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"buf": buf, "reservation_id": resID}).Error("Error unmarshaling job")
			return
		}

		_, err = redisPush(conn, mq.queueName, &job)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"reservation_id": resID}).Error("Pushing delayed job")
			return
		}
		conn.Do("HDEL", mq.k("delayed_jobs"), resID)
	}

	// Remove everything we processed.
	conn.Do("ZREMRANGEBYSCORE", mq.k("delays"), "-inf", now)
}

func (mq *RedisMQ) start() {
	go func() {
		for range mq.ticker.C {
			mq.processPendingReservations()
			mq.processDelayedCalls()
		}
	}()
}

func redisPush(conn redis.Conn, queue string, job *models.Call) (*models.Call, error) {
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

func (mq *RedisMQ) delayCall(conn redis.Conn, job *models.Call) (*models.Call, error) {
	buf, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}

	resp, err := redis.Int64(conn.Do("INCR", mq.k("delays_counter")))
	if err != nil {
		return nil, err
	}

	reservationID := strconv.FormatInt(resp, 10)

	// Timestamp -> resID
	_, err = conn.Do("ZADD", mq.k("delays"), time.Now().UTC().Add(time.Duration(job.Delay)*time.Second).Unix(), reservationID)
	if err != nil {
		return nil, err
	}

	// resID -> Task
	_, err = conn.Do("HSET", mq.k("delayed_jobs"), reservationID, buf)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (mq *RedisMQ) Push(ctx context.Context, job *models.Call) (*models.Call, error) {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	defer log.Debugln("Pushed to MQ")

	conn := mq.pool.Get()
	defer conn.Close()

	if job.Delay > 0 {
		return mq.delayCall(conn, job)
	}
	return redisPush(conn, mq.queueName, job)
}
func (mq *RedisMQ) checkNilResponse(err error) bool {
	return err != nil && err.Error() == redis.ErrNil.Error()
}

// Would be nice to switch to this model http://redis.io/commands/rpoplpush#pattern-reliable-queue
func (mq *RedisMQ) Reserve(ctx context.Context) (*models.Call, error) {

	conn := mq.pool.Get()
	defer conn.Close()
	var job models.Call
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
	reservationID := strconv.FormatInt(response, 10)
	_, err = conn.Do("ZADD", "timeout:", time.Now().Add(time.Minute).Unix(), reservationID)
	if err != nil {
		return nil, err
	}
	_, err = conn.Do("HSET", "timeout", reservationID, resp)
	if err != nil {
		return nil, err
	}

	// Map from job.ID -> reservation ID
	_, err = conn.Do("HSET", "reservations", job.ID, reservationID)
	if err != nil {
		return nil, err
	}

	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	log.Debugln("Reserved")

	return &job, nil
}

func (mq *RedisMQ) Delete(ctx context.Context, job *models.Call) error {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	defer log.Debugln("Deleted")

	conn := mq.pool.Get()
	defer conn.Close()
	resID, err := conn.Do("HGET", "reservations", job.ID)
	if err != nil {
		return err
	}
	_, err = conn.Do("HDEL", "reservations", job.ID)
	if err != nil {
		return err
	}
	_, err = conn.Do("ZREM", "timeout:", resID)
	if err != nil {
		return err
	}
	_, err = conn.Do("HDEL", "timeout", resID)
	return err
}

// Close shuts down the redis connection pool and
// stops the goroutine associated with the ticker
func (mq *RedisMQ) Close() error {
	mq.ticker.Stop()
	return mq.pool.Close()
}

func init() {
	mqs.AddProvider(redisProvider(0))
}
