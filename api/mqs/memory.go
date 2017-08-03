package mqs

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/google/btree"
	"github.com/sirupsen/logrus"
)

type MemoryMQ struct {
	// WorkQueue A buffered channel that we can send work requests on.
	PriorityQueues []chan *models.Call
	Ticker         *time.Ticker
	BTree          *btree.BTree
	Timeouts       map[string]*callItem
	// Protects B-tree and Timeouts
	// If this becomes a bottleneck, consider separating the two mutexes. The
	// goroutine to clear up timed out messages could also become a bottleneck at
	// some point. May need to switch to bucketing of some sort.
	Mutex sync.Mutex
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	rand.Seed(time.Now().Unix())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

const NumPriorities = 3

func NewMemoryMQ() *MemoryMQ {
	var queues []chan *models.Call
	for i := 0; i < NumPriorities; i++ {
		queues = append(queues, make(chan *models.Call, 5000))
	}
	ticker := time.NewTicker(time.Second)
	mq := &MemoryMQ{
		PriorityQueues: queues,
		Ticker:         ticker,
		BTree:          btree.New(2),
		Timeouts:       make(map[string]*callItem, 0),
	}
	mq.start()
	logrus.Info("MemoryMQ initialized")
	return mq
}

func (mq *MemoryMQ) start() {
	// start goroutine to check for delayed jobs and put them onto regular queue when ready
	go func() {
		for range mq.Ticker.C {
			ji := &callItem{
				StartAt: time.Now(),
			}
			mq.Mutex.Lock()
			mq.BTree.AscendLessThan(ji, func(a btree.Item) bool {
				logrus.WithFields(logrus.Fields{"queue": a}).Debug("delayed job move to queue")
				ji2 := mq.BTree.Delete(a).(*callItem)
				// put it onto the regular queue now
				_, err := mq.pushForce(ji2.Call)
				if err != nil {
					logrus.WithError(err).Error("Couldn't push delayed message onto main queue")
				}
				return true
			})
			mq.Mutex.Unlock()
		}
	}()
	// start goroutine to check for messages that have timed out and put them back onto regular queue
	// TODO: this should be like the delayed messages above. Could even be the same thing as delayed messages, but remove them if job is completed.
	go func() {
		for range mq.Ticker.C {
			ji := &callItem{
				StartAt: time.Now(),
			}
			mq.Mutex.Lock()
			for _, jobItem := range mq.Timeouts {
				if jobItem.Less(ji) {
					delete(mq.Timeouts, jobItem.Call.ID)
					_, err := mq.pushForce(jobItem.Call)
					if err != nil {
						logrus.WithError(err).Error("Couldn't push timed out message onto main queue")
					}
				}
			}
			mq.Mutex.Unlock()
		}
	}()
}

// callItem is for the Btree, implements btree.Item
type callItem struct {
	Call    *models.Call
	StartAt time.Time
}

func (ji *callItem) Less(than btree.Item) bool {
	// TODO: this could lose jobs: https://godoc.org/github.com/google/btree#Item
	ji2 := than.(*callItem)
	return ji.StartAt.Before(ji2.StartAt)
}

func (mq *MemoryMQ) Push(ctx context.Context, job *models.Call) (*models.Call, error) {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	log.Println("Pushed to MQ")

	// It seems to me that using the job ID in the reservation is acceptable since each job can only have one outstanding reservation.
	// job.MsgId = randSeq(20)
	if job.Delay > 0 {
		// then we'll put into short term storage until ready
		ji := &callItem{
			Call:    job,
			StartAt: time.Now().Add(time.Second * time.Duration(job.Delay)),
		}
		mq.Mutex.Lock()
		replaced := mq.BTree.ReplaceOrInsert(ji)
		mq.Mutex.Unlock()
		if replaced != nil {
			log.Warn("Ooops! an item was replaced and therefore lost, not good.")
		}
		return job, nil
	}

	// Push the work onto the queue.
	return mq.pushForce(job)
}

func (mq *MemoryMQ) pushTimeout(job *models.Call) error {

	ji := &callItem{
		Call:    job,
		StartAt: time.Now().Add(time.Minute),
	}
	mq.Mutex.Lock()
	mq.Timeouts[job.ID] = ji
	mq.Mutex.Unlock()
	return nil
}

func (mq *MemoryMQ) pushForce(job *models.Call) (*models.Call, error) {
	mq.PriorityQueues[*job.Priority] <- job
	return job, nil
}

// This is recursive, so be careful how many channels you pass in.
func pickEarliestNonblocking(channels ...chan *models.Call) *models.Call {
	if len(channels) == 0 {
		return nil
	}

	select {
	case job := <-channels[0]:
		return job
	default:
		return pickEarliestNonblocking(channels[1:]...)
	}
}

func (mq *MemoryMQ) Reserve(ctx context.Context) (*models.Call, error) {
	job := pickEarliestNonblocking(mq.PriorityQueues[2], mq.PriorityQueues[1], mq.PriorityQueues[0])
	if job == nil {
		return nil, nil
	}

	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})
	log.Println("Reserved")
	return job, mq.pushTimeout(job)
}

func (mq *MemoryMQ) Delete(ctx context.Context, job *models.Call) error {
	_, log := common.LoggerWithFields(ctx, logrus.Fields{"call_id": job.ID})

	mq.Mutex.Lock()
	defer mq.Mutex.Unlock()
	_, exists := mq.Timeouts[job.ID]
	if !exists {
		return errors.New("Not reserved")
	}

	delete(mq.Timeouts, job.ID)
	log.Println("Deleted")
	return nil
}
