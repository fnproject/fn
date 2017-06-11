package mq_test

import (
	"errors"

	"github.com/iron-io/iron_go3/mq"
)

func ExampleQueue() error {
	// Standard way of using a queue will be to just start pushing or
	// getting messages, q.Upsert isn't necessary unless you explicitly
	// need to create a queue with custom settings.

	q := mq.New("my_queue2")
	// Simply pushing messages will create a queue if it doesn't exist, with defaults.
	_, err := q.PushStrings("msg1", "msg2")
	if err != nil {
		return err
	}
	msgs, err := q.GetN(2)
	if err != nil {
		return err
	}
	if len(msgs) != 2 {
		return errors.New("not good")
	}

	return nil
}

func ExampleQueue_Upsert() error {
	// Prepare a Queue from configs
	q := mq.New("my_queue")
	// Upsert will create the queue on the server or update its message_timeout
	// to 120 if it already exists.

	// Let's just make sure we don't have a queue, because we can.
	if _, err := q.Info(); mq.ErrQueueNotFound(err) {
		_, err := q.Update(mq.QueueInfo{MessageTimeout: 120}) // ok, we'll make one.
		if err != nil {
			return err
		}
	}
	// Definitely exists now.

	// Let's just add some messages.
	_, err := q.PushStrings("msg1", "msg2")
	if err != nil {
		return err
	}
	msgs, err := q.Peek()
	if len(msgs) != 2 {
		// and it has messages already...
	}
	return nil
}

func ExampleList() error {
	qs, err := mq.List() // Will get up to 30 queues. All ready to use.
	if err != nil {
		return err
	}

	// Pop a message off of each queue.
	for _, q := range qs {
		_, err := q.Pop()
		if err != nil {
			return err
		}
	}
	return nil
}
