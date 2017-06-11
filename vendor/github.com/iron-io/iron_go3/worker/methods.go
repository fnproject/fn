package worker

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"time"
)

type Schedule struct {
	CodeName       string         `json:"code_name"`
	Delay          *time.Duration `json:"delay"`
	EndAt          *time.Time     `json:"end_at"`
	MaxConcurrency *int           `json:"max_concurrency"`
	Name           string         `json:"name"`
	Payload        string         `json:"payload"`
	Priority       *int           `json:"priority"`
	RunEvery       *int           `json:"run_every"`
	RunTimes       *int           `json:"run_times"`
	StartAt        *time.Time     `json:"start_at"`
	Cluster        string         `json:"cluster"`
	Label          string         `json:"label"`
}

type ScheduleInfo struct {
	CodeName       string    `json:"code_name"`
	CreatedAt      time.Time `json:"created_at"`
	EndAt          time.Time `json:"end_at"`
	Id             string    `json:"id"`
	LastRunTime    time.Time `json:"last_run_time"`
	MaxConcurrency int       `json:"max_concurrency"`
	Msg            string    `json:"msg"`
	NextStart      time.Time `json:"next_start"`
	ProjectId      string    `json:"project_id"`
	RunCount       int       `json:"run_count"`
	RunTimes       int       `json:"run_times"`
	StartAt        time.Time `json:"start_at"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Task struct {
	CodeName string         `json:"code_name"`
	Payload  string         `json:"payload"`
	Priority int            `json:"priority"`
	Timeout  *time.Duration `json:"timeout"`
	Delay    *time.Duration `json:"delay"`
	Cluster  string         `json:"cluster"`
	Label    string         `json:"label"`
}

type TaskInfo struct {
	CodeHistoryId string    `json:"code_history_id"`
	CodeId        string    `json:"code_id"`
	CodeName      string    `json:"code_name"`
	CodeRev       string    `json:"code_rev"`
	Id            string    `json:"id"`
	Payload       string    `json:"payload"`
	ProjectId     string    `json:"project_id"`
	Status        string    `json:"status"`
	Msg           string    `json:"msg,omitempty"`
	ScheduleId    string    `json:"schedule_id"`
	Duration      int       `json:"duration"`
	RunTimes      int       `json:"run_times"`
	Timeout       int       `json:"timeout"`
	Percent       int       `json:"percent,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
}

type CodeSource map[string][]byte // map[pathInZip]code

type Code struct {
	Id              string            `json:"id,omitempty"`
	Name            string            `json:"name"`
	Runtime         string            `json:"runtime"`
	FileName        string            `json:"file_name"`
	Config          string            `json:"config,omitempty"`
	MaxConcurrency  int               `json:"max_concurrency,omitempty"`
	Retries         *int              `json:"retries,omitempty"`
	RetriesDelay    *int              `json:"retries_delay,omitempty"` // seconds
	Stack           string            `json:"stack"`
	Image           string            `json:"image"`
	Command         string            `json:"command"`
	Host            string            `json:"host,omitempty"` // PaaS router thing
	EnvVars         map[string]string `json:"env_vars"`
	Source          CodeSource        `json:"-"`
	DefaultPriority int               `json:"default_priority,omitempty"`
}

type CodeInfo struct {
	Id              string    `json:"id"`
	LatestChecksum  string    `json:"latest_checksum"`
	LatestHistoryId string    `json:"latest_history_id"`
	Name            string    `json:"name"`
	ProjectId       string    `json:"project_id"`
	Runtime         *string   `json:"runtime"`
	Rev             int       `json:"rev"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LatestChange    time.Time `json:"latest_change"`
}

// CodePackageList lists code packages.
//
// The page argument decides the page of code packages you want to retrieve, starting from 0, maximum is 100.
//
// The perPage argument determines the number of code packages to return. Note
// this is a maximum value, so there may be fewer packages returned if there
// aren’t enough results. If this is < 1, 1 will be the default. Maximum is 100.
func (w *Worker) CodePackageList(page, perPage int) (codes []CodeInfo, err error) {
	out := map[string][]CodeInfo{}

	err = w.codes().
		QueryAdd("page", "%d", page).
		QueryAdd("per_page", "%d", perPage).
		Req("GET", nil, &out)
	if err != nil {
		return
	}

	return out["codes"], nil
}

// CodePackageInfo gets info about a code package
func (w *Worker) CodePackageInfo(codeId string) (code CodeInfo, err error) {
	out := CodeInfo{}
	err = w.codes(codeId).Req("GET", nil, &out)
	return out, err
}

// CodePackageDelete deletes a code package
func (w *Worker) CodePackageDelete(codeId string) (err error) {
	return w.codes(codeId).Req("DELETE", nil, nil)
}

// CodePackageDownload downloads a code package
func (w *Worker) CodePackageDownload(codeId string) (code Code, err error) {
	out := Code{}
	err = w.codes(codeId, "download").Req("GET", nil, &out)
	return out, err
}

// CodePackageRevisions lists the revisions of a code pacakge
func (w *Worker) CodePackageRevisions(codeId string) (code Code, err error) {
	out := Code{}
	err = w.codes(codeId, "revisions").Req("GET", nil, &out)
	return out, err
}

// CodePackageZipUpload can be used to upload a code package with a zip
// package, where zipName is a filepath where the zip can be located.  If
// zipName is an empty string, then the code package will be uploaded without a
// zip package (see CodePackageUpload).
func (w *Worker) CodePackageZipUpload(zipName string, args Code) (*Code, error) {
	return w.codePackageUpload(zipName, args)
}

// CodePackageUpload uploads a code package without a zip file.
func (w *Worker) CodePackageUpload(args Code) (*Code, error) {
	return w.codePackageUpload("", args)
}

func (w *Worker) codePackageUpload(zipName string, args Code) (*Code, error) {
	b := randomBoundary()
	r := &streamZipPipe{zipName: zipName, args: args, boundary: b}
	defer r.Close()

	var out Code
	err := w.codes().
		SetContentType("multipart/form-data; boundary="+b).
		Req("POST", r, &out)

	return &out, err
}

func randomBoundary() string {
	var buf [30]byte
	_, err := io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", buf[:])
}

// implement seek so that we can retry it. not thread safe,
// Read and Seek must be called in same thread.
type streamZipPipe struct {
	zipName  string
	args     Code
	boundary string

	r    io.ReadCloser
	w    io.WriteCloser
	once bool
	err  chan error
}

// safe to call multiple times, implement io.Closer so http will call this
func (s *streamZipPipe) Close() error {
	if s.r != nil {
		return s.r.Close()
	}
	return nil
}

// only seeks to beginning, ignores parameters
func (s *streamZipPipe) Seek(offset int64, whence int) (int64, error) {
	// just restart the whole thing, the last pipe should have errored out and been closed
	s.r, s.w = io.Pipe()
	s.err = make(chan error, 1)
	s.once = true
	go s.pipe()
	return 0, nil
}

func (s *streamZipPipe) Read(b []byte) (int, error) {
	if !s.once {
		s.once = true
		s.r, s.w = io.Pipe()
		s.err = make(chan error, 1)
		go s.pipe()
	}

	select {
	case err := <-s.err:
		if err != nil {
			return 0, err // n should get ignored
		}
	default:
	}
	return s.r.Read(b)
}

func (s *streamZipPipe) pipe() {
	defer s.w.Close()

	mWriter := multipart.NewWriter(s.w)
	mWriter.SetBoundary(s.boundary)
	mMetaWriter, err := mWriter.CreateFormField("data")
	if err != nil {
		s.err <- err
		return
	}

	if err := json.NewEncoder(mMetaWriter).Encode(s.args); err != nil {
		s.err <- err
		return
	}

	if s.zipName != "" {
		r, err := zip.OpenReader(s.zipName)
		if err != nil {
			s.err <- err
			return
		}
		defer r.Close()

		mFileWriter, err := mWriter.CreateFormFile("file", "worker.zip")
		if err != nil {
			s.err <- err
			return
		}
		zWriter := zip.NewWriter(mFileWriter)

		for _, f := range r.File {
			fWriter, err := zWriter.Create(f.Name)
			if err != nil {
				s.err <- err
				return
			}
			rc, err := f.Open()
			if err != nil {
				s.err <- err
				return
			}
			_, err = io.Copy(fWriter, rc)
			rc.Close()
			if err != nil {
				s.err <- err
				return
			}
		}

		if err := zWriter.Close(); err != nil {
			s.err <- err
			return
		}
	}
	if err := mWriter.Close(); err != nil {
		s.err <- err
	}
}

func (w *Worker) TaskList() (tasks []TaskInfo, err error) {
	out := map[string][]TaskInfo{}
	err = w.tasks().Req("GET", nil, &out)
	if err != nil {
		return
	}
	return out["tasks"], nil
}

type TaskListParams struct {
	CodeName string
	Label    string
	Page     int
	PerPage  int
	FromTime time.Time
	ToTime   time.Time
	Statuses []string
}

func (w *Worker) FilteredTaskList(params TaskListParams) (tasks []TaskInfo, err error) {
	out := map[string][]TaskInfo{}
	url := w.tasks()

	url.QueryAdd("code_name", "%s", params.CodeName)

	if params.Label != "" {
		url.QueryAdd("label", "%s", params.Label)
	}

	if params.Page > 0 {
		url.QueryAdd("page", "%d", params.Page)
	}

	if params.PerPage > 0 {
		url.QueryAdd("per_page", "%d", params.PerPage)
	}

	if fromTimeSeconds := params.FromTime.Unix(); fromTimeSeconds > 0 {
		url.QueryAdd("from_time", "%d", fromTimeSeconds)
	}

	if toTimeSeconds := params.ToTime.Unix(); toTimeSeconds > 0 {
		url.QueryAdd("to_time", "%d", toTimeSeconds)
	}

	for _, status := range params.Statuses {
		url.QueryAdd(status, "%d", true)
	}

	err = url.Req("GET", nil, &out)

	if err != nil {
		return
	}

	return out["tasks"], nil
}

// TaskQueue queues a task
func (w *Worker) TaskQueue(tasks ...Task) (taskIds []string, err error) {
	outTasks := make([]map[string]interface{}, 0, len(tasks))

	for _, task := range tasks {
		thisTask := map[string]interface{}{
			"code_name": task.CodeName,
			"payload":   task.Payload,
			"priority":  task.Priority,
			"cluster":   task.Cluster,
			"label":     task.Label,
		}
		if task.Timeout != nil {
			thisTask["timeout"] = (*task.Timeout).Seconds()
		}
		if task.Delay != nil {
			thisTask["delay"] = int64((*task.Delay).Seconds())
		}

		outTasks = append(outTasks, thisTask)
	}

	in := map[string][]map[string]interface{}{"tasks": outTasks}
	out := struct {
		Tasks []struct {
			Id string `json:"id"`
		} `json:"tasks"`
		Msg string `json:"msg"`
	}{}

	err = w.tasks().Req("POST", &in, &out)
	if err != nil {
		return
	}

	taskIds = make([]string, 0, len(out.Tasks))
	for _, task := range out.Tasks {
		taskIds = append(taskIds, task.Id)
	}

	return
}

// TaskInfo gives info about a given task
func (w *Worker) TaskInfo(taskId string) (task TaskInfo, err error) {
	out := TaskInfo{}
	err = w.tasks(taskId).Req("GET", nil, &out)
	return out, err
}

func (w *Worker) TaskLog(taskId string) (log []byte, err error) {
	response, err := w.tasks(taskId, "log").Request("GET", nil)
	if err != nil {
		return
	}

	log, err = ioutil.ReadAll(response.Body)
	return
}

// TaskCancel cancels a Task
func (w *Worker) TaskCancel(taskId string) (err error) {
	_, err = w.tasks(taskId, "cancel").Request("POST", nil)
	return err
}

// TaskProgress sets a Task's Progress
func (w *Worker) TaskProgress(taskId string, progress int, msg string) (err error) {
	payload := map[string]interface{}{
		"msg":     msg,
		"percent": progress,
	}

	err = w.tasks(taskId, "progress").Req("POST", payload, nil)
	return
}

// TaskQueueWebhook queues a Task from a Webhook
func (w *Worker) TaskQueueWebhook() (err error) { return }

// ScheduleList lists Scheduled Tasks
func (w *Worker) ScheduleList() (schedules []ScheduleInfo, err error) {
	out := map[string][]ScheduleInfo{}
	err = w.schedules().Req("GET", nil, &out)
	if err != nil {
		return
	}
	return out["schedules"], nil
}

// Schedule a Task
func (w *Worker) Schedule(schedules ...Schedule) (scheduleIds []string, err error) {
	outSchedules := make([]map[string]interface{}, 0, len(schedules))

	for _, schedule := range schedules {
		sm := map[string]interface{}{
			"code_name": schedule.CodeName,
			"name":      schedule.Name,
			"payload":   schedule.Payload,
			"label":     schedule.Label,
			"cluster":   schedule.Cluster,
		}
		if schedule.Delay != nil {
			sm["delay"] = (*schedule.Delay).Seconds()
		}
		if schedule.EndAt != nil {
			sm["end_at"] = *schedule.EndAt
		}
		if schedule.MaxConcurrency != nil {
			sm["max_concurrency"] = *schedule.MaxConcurrency
		}
		if schedule.Priority != nil {
			sm["priority"] = *schedule.Priority
		}
		if schedule.RunEvery != nil {
			sm["run_every"] = *schedule.RunEvery
		}
		if schedule.RunTimes != nil {
			sm["run_times"] = *schedule.RunTimes
		}
		if schedule.StartAt != nil {
			sm["start_at"] = *schedule.StartAt
		}
		outSchedules = append(outSchedules, sm)
	}

	in := map[string][]map[string]interface{}{"schedules": outSchedules}
	out := struct {
		Schedules []struct {
			Id string `json:"id"`
		} `json:"schedules"`
		Msg string `json:"msg"`
	}{}

	err = w.schedules().Req("POST", &in, &out)
	if err != nil {
		return
	}

	scheduleIds = make([]string, 0, len(out.Schedules))

	for _, schedule := range out.Schedules {
		scheduleIds = append(scheduleIds, schedule.Id)
	}

	return
}

// ScheduleInfo gets info about a scheduled task
func (w *Worker) ScheduleInfo(scheduleId string) (info ScheduleInfo, err error) {
	info = ScheduleInfo{}
	err = w.schedules(scheduleId).Req("GET", nil, &info)
	return info, nil
}

// ScheduleCancel cancels a scheduled task
func (w *Worker) ScheduleCancel(scheduleId string) (err error) {
	_, err = w.schedules(scheduleId, "cancel").Request("POST", nil)
	return
}

// TODO we should probably support other crypto functions at some point so that people have a choice.

// - expects an x509 rsa public key (ala "-----BEGIN RSA PUBLIC KEY-----")
// - returns a base64 ciphertext with an rsa encrypted aes-128 session key stored in the bit length
//   of the modulus of the given RSA key first bits (i.e. 2048 = first 256 bytes), followed by
//   the aes cipher with a new, random iv in the first 12 bytes,
//   and the auth tag in the last 16 bytes of the cipher.
// - must have RSA key >= 1024
// - end format w/ RSA of 2048 for display purposes, all base64 encoded:
//   [ 256 byte RSA encrypted AES key | len(payload) AES-GCM cipher | 16 bytes AES tag | 12 bytes AES nonce ]
// - each task will be encrypted with a different AES session key
//
// EncryptPayloads will return a copy of the input tasks with the Payload field modified
// to be encrypted as described above. Upon any error, the tasks returned will be nil.
func EncryptPayloads(publicKey []byte, in ...Task) ([]Task, error) {
	rsablock, _ := pem.Decode(publicKey)
	rsaKey, err := x509.ParsePKIXPublicKey(rsablock.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPublicKey := rsaKey.(*rsa.PublicKey)

	tasks := make([]Task, len(in))
	copy(tasks, in)

	for i := range tasks {
		// get a random aes-128 session key to encrypt
		aesKey := make([]byte, 128/8)
		if _, err := rand.Read(aesKey); err != nil {
			return nil, err
		}

		// have to use sha1 b/c ruby openssl picks it for OAEP:  https://www.openssl.org/docs/manmaster/crypto/RSA_public_encrypt.html
		aesKeyCipher, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, rsaPublicKey, aesKey, nil)
		if err != nil {
			return nil, err
		}

		block, err := aes.NewCipher(aesKey)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		pbytes := []byte(tasks[i].Payload)
		// The IV needs to be unique, but not secure. last 12 bytes are IV.
		ciphertext := make([]byte, len(pbytes)+gcm.Overhead()+gcm.NonceSize())
		nonce := ciphertext[len(ciphertext)-gcm.NonceSize():]
		if _, err := rand.Read(nonce); err != nil {
			return nil, err
		}
		// tag is appended to cipher as last 16 bytes. https://golang.org/src/crypto/cipher/gcm.go?s=2318:2357#L145
		gcm.Seal(ciphertext[:0], nonce, pbytes, nil)

		// base64 the whole thing
		tasks[i].Payload = base64.StdEncoding.EncodeToString(append(aesKeyCipher, ciphertext...))
	}
	return tasks, nil
}

type Cluster struct {
	Id        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Memory    int64  `json:"memory,omitempty"`
	DiskSpace int64  `json:"disk_space,omitempty"`
	CpuShare  *int32 `json:"cpu_share,omitempty"`
}

func (w *Worker) ClusterCreate(c Cluster) (Cluster, error) {
	var out struct {
		C Cluster `json:"cluster"`
	}
	err := w.clusters().Req("POST", c, &out)
	return out.C, err
}

func (w *Worker) ClusterDelete(id string) error {
	return w.clusters(id).Req("DELETE", nil, nil)
}

func (w *Worker) ClusterToken(id string) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	err := w.clusters(id, "credentials").Req("GET", nil, &out)
	return out.Token, err
}
