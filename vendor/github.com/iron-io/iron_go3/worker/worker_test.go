package worker

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	// "github.com/iron-io/iron_go/worker"
	. "github.com/jeffh/go.bdd"
)

func TestEverything(*testing.T) {
	defer PrintSpecReport()

	Describe("iron.io worker", func() {
		w := New()

		It("Prepares the specs by deleting all existing code packages", func() {
			codes, err := w.CodePackageList(0, 100)
			Expect(err, ToBeNil)
			for _, code := range codes {
				err = w.CodePackageDelete(code.Id)
				Expect(err, ToBeNil)
			}

			codes, err = w.CodePackageList(0, 100)
			Expect(err, ToBeNil)
			Expect(len(codes), ToEqual, 0)
		})

		It("Creates a code package", func() {
			tempDir, err := ioutil.TempDir("", "iron-worker")
			Expect(err, ToBeNil)
			defer os.RemoveAll(tempDir)

			fd, err := os.Create(tempDir + "/main.go")
			Expect(err, ToBeNil)

			n, err := fd.WriteString(`package main; func main(){ println("Hello world!") }`)
			Expect(err, ToBeNil)
			Expect(n, ToEqual, 52)

			Expect(fd.Close(), ToBeNil)

			pkg, err := NewGoCodePackage("GoFun", fd.Name())
			Expect(err, ToBeNil)

			id, err := w.CodePackageUpload(pkg)
			Expect(err, ToBeNil)

			info, err := w.CodePackageInfo(id)
			Expect(err, ToBeNil)
			Expect(info.Id, ToEqual, id)
			Expect(info.Name, ToEqual, "GoFun")
			Expect(info.Rev, ToEqual, 1)
		})

		It("Queues a Task", func() {
			ids, err := w.TaskQueue(Task{CodeName: "GoFun"})
			Expect(err, ToBeNil)

			id := ids[0]
			info, err := w.TaskInfo(id)
			Expect(err, ToBeNil)
			Expect(info.CodeName, ToEqual, "GoFun")

			select {
			case info = <-w.WaitForTask(id):
				Expect(info.Status, ToEqual, "complete")
			case <-time.After(5 * time.Second):
				panic("info timed out")
			}

			log, err := w.TaskLog(id)
			Expect(err, ToBeNil)
			Expect(log, ToDeepEqual, []byte("Hello world!\n"))
		})

		It("Cancels a task", func() {
			delay := 10 * time.Second
			ids, err := w.TaskQueue(Task{CodeName: "GoFun", Delay: &delay})
			Expect(err, ToBeNil)

			id := ids[0]
			err = w.TaskCancel(id)
			Expect(err, ToBeNil)

			info, err := w.TaskInfo(id)
			Expect(info.Status, ToEqual, "cancelled")
		})

		It("Queues a lot of tasks and lists them", func() {
			delay := 100 * time.Second
			ids, err := w.TaskQueue(Task{CodeName: "GoFun", Delay: &delay})
			Expect(err, ToBeNil)
			firstId := ids[0]
			time.Sleep(1 * time.Second)

			ids, err = w.TaskQueue(Task{CodeName: "GoFun", Delay: &delay})
			Expect(err, ToBeNil)
			secondId := ids[0]

			tasks, err := w.TaskList()
			Expect(err, ToBeNil)

			Expect(tasks[0].CreatedAt.After(tasks[1].CreatedAt), ToEqual, true)
			Expect(tasks[0].Id, ToEqual, secondId)
			Expect(tasks[1].Id, ToEqual, firstId)
		})

		It("Schedules a Task ", func() {
			delay := 10 * time.Second
			ids, err := w.Schedule(Schedule{
				Name:     "ScheduledGoFun",
				CodeName: "GoFun",
				Payload:  "foobar",
				Delay:    &delay,
			})

			Expect(err, ToBeNil)
			id := ids[0]

			info, err := w.ScheduleInfo(id)
			Expect(err, ToBeNil)
			Expect(info.CodeName, ToEqual, "GoFun")
			Expect(info.Status, ToEqual, "scheduled")
		})

		It("Cancels a scheduled task", func() {
			delay := 10 * time.Second
			ids, err := w.Schedule(Schedule{
				Name:     "ScheduledGoFun",
				CodeName: "GoFun",
				Payload:  "foobar",
				Delay:    &delay,
			})

			Expect(err, ToBeNil)
			id := ids[0]

			err = w.ScheduleCancel(id)
			Expect(err, ToBeNil)

			info, err := w.ScheduleInfo(id)
			Expect(err, ToBeNil)
			Expect(info.CodeName, ToEqual, "GoFun")
			Expect(info.Status, ToEqual, "cancelled")
		})
	})
}
