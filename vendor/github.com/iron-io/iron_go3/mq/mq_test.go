package mq

import (
	"fmt"
	"testing"
	"time"

	. "github.com/jeffh/go.bdd"
)

func TestEverything(t *testing.T) {}

func q(name string) Queue {
	c := New(name)
	return c
}

func init() {
	defer PrintSpecReport()

	Describe("IronMQ", func() {
		It("Deletes all existing messages", func() {
			c := q("queuename")

			_, err := c.PushString("just a little test")
			Expect(err, ToBeNil)

			Expect(c.Clear(), ToBeNil)

			info, err := c.Info()
			Expect(err, ToBeNil)
			Expect(info.Size, ToEqual, 0x0)
		})

		It("Pushes ands gets a message", func() {
			c := q("queuename")
			id1, err := c.PushString("just a little test")
			Expect(err, ToBeNil)

			msg, err := c.Get()
			Expect(err, ToBeNil)

			Expect(msg, ToNotBeNil)
			Expect(msg.Id, ToDeepEqual, id1)
			Expect(msg.Body, ToDeepEqual, "just a little test")

			err = c.DeleteMessage(msg.Id, msg.ReservationId)
			Expect(err, ToBeNil)

			info, err := c.Info()
			Expect(err, ToBeNil)
			Expect(info.Size, ToEqual, 0x0)

		})

		It("clears the queue", func() {
			q := q("queuename")

			strings := []string{}
			for n := 0; n < 100; n++ {
				strings = append(strings, fmt.Sprint("test: ", n))
			}

			_, err := q.PushStrings(strings...)
			Expect(err, ToBeNil)

			info, err := q.Info()
			Expect(err, ToBeNil)
			Expect(info.Size, ToEqual, 100)

			Expect(q.Clear(), ToBeNil)

			info, err = q.Info()
			Expect(err, ToBeNil)
			Expect(info.Size, ToEqual, 0)
		})

		It("Lists all queues", func() {
			c := q("queuename")
			queues, err := ListQueues(c.Settings, "", "", 101) // can't check the caches value just yet.
			Expect(err, ToBeNil)
			l := len(queues)
			t := l >= 1
			Expect(t, ToBeTrue)
			found := false
			for _, queue := range queues {
				if queue.Name == "queuename" {
					found = true
					break
				}
			}
			Expect(found, ToEqual, true)
		})

		It("releases a message", func() {
			c := q("queuename")

			id, err := c.PushString("trying")
			Expect(err, ToBeNil)

			msg, err := c.Get()
			Expect(err, ToBeNil)

			err = msg.Release(3)
			Expect(err, ToBeNil)

			msg, err = c.Get()
			Expect(msg, ToBeNil)

			time.Sleep(4 * time.Second)

			msg, err = c.Get()
			Expect(err, ToBeNil)
			Expect(msg, ToNotBeNil)
			Expect(msg.Id, ToEqual, id)
		})

		It("updates a queue", func() {
			name := "pushqueue" + time.Now().String()

			_, err := CreateQueue(name, QueueInfo{Type: "multicast", Push: &PushInfo{
				Subscribers: []QueueSubscriber{{Name: "first", URL: "http://hit.me.with.a.message"}}}})
			Expect(err, ToBeNil)

			c := q(name)

			info, err := c.Info()
			Expect(err, ToBeNil)

			qi := QueueInfo{Type: "multicast", Push: &PushInfo{
				Subscribers: []QueueSubscriber{{Name: "first", URL: "http://hit.me.with.another.message"}}}}
			rc, err := c.Update(qi)
			Expect(err, ToBeNil)
			info, err = c.Info()
			Expect(err, ToBeNil)
			Expect(info.Name, ToEqual, rc.Name)

			err = c.Delete()
			Expect(err, ToBeNil)
		})
	})
}
