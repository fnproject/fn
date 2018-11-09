package tests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
)

func TestCanExecuteFunction(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, models.TypeSync)
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	echo, err := getEchoContent(output.Bytes())
	if err != nil || echo != "HelloWorld" {
		t.Fatalf("getEchoContent/HelloWorld check failed on %v", output)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}

	// Now let's check FN_CHEESE, since LB and runners have override/extension mechanism
	// to insert FN_CHEESE into config
	cheese, err := getConfigContent("FN_CHEESE", output.Bytes())
	if err != nil || cheese != "Tete de Moine" {
		t.Fatalf("getConfigContent/FN_CHEESE check failed (%v) on %v", err, output)
	}

	// Now let's check FN_WINE, since runners have override to insert this.
	wine, err := getConfigContent("FN_WINE", output.Bytes())
	if err != nil || wine != "1982 Margaux" {
		t.Fatalf("getConfigContent/FN_WINE check failed (%v) on %v", err, output)
	}
}

func TestCanExecuteDetachedFunction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, models.TypeDetached)
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestCanExecuteBigOutput(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	// Approx 5.3MB output
	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 410000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, models.TypeSync)
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	t.Logf("getEchoContent/HelloWorld size %d", len(output.Bytes()))

	echo, err := getEchoContent(output.Bytes())
	if err != nil || echo != "HelloWorld" {
		t.Fatalf("getEchoContent/HelloWorld check failed on %v", output)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestCanExecuteTooBigOutput(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	// > 6MB output
	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 600000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, models.TypeSync)
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	exp := "{\"message\":\"function response too large\"}\n"
	actual := output.String()

	if !strings.Contains(exp, actual) || len(exp) != len(actual) {
		t.Fatalf("Assertion error.\n\tExpected: %v\n\tActual: %v", exp, output.String())
	}

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestCanExecuteEmptyOutput(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	// empty body output
	body := `{"sleepTime": 0, "isDebug": true, "isEmptyBody": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, models.TypeSync)
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	actual := output.String()

	if 0 != len(actual) {
		t.Fatalf("Assertion error.\n\tExpected empty\n\tActual: %v", output.String())
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestBasicConcurrentExecution(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	results := make(chan error)
	latch := make(chan struct{})
	concurrentFuncs := 10
	for i := 0; i < concurrentFuncs; i++ {
		go func() {
			body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
			content := bytes.NewBuffer([]byte(body))
			output := &bytes.Buffer{}
			<-latch
			resp, err := callFN(ctx, u.String(), content, output, models.TypeSync)
			if err != nil {
				results <- fmt.Errorf("Got unexpected error: %v", err)
				return
			}

			echo, err := getEchoContent(output.Bytes())
			if err != nil || echo != "HelloWorld" {
				results <- fmt.Errorf("Assertion error.\n\tActual: %v", output.String())
				return
			}
			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("StatusCode check failed on %v", resp.StatusCode)
				return
			}

			results <- nil
		}()
	}
	close(latch)
	for i := 0; i < concurrentFuncs; i++ {
		err := <-results
		if err != nil {
			t.Fatalf("Error in basic concurrency execution test: %v", err)
		}
	}
}

func TestBasicConcurrentDetachedExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: image,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	results := make(chan error)
	latch := make(chan struct{})
	concurrentFuncs := 10
	for i := 0; i < concurrentFuncs; i++ {
		go func() {
			body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
			content := bytes.NewBuffer([]byte(body))
			output := &bytes.Buffer{}
			<-latch
			resp, err := callFN(ctx, u.String(), content, output, models.TypeDetached)
			if err != nil {
				results <- fmt.Errorf("Got unexpected error: %v", err)
				return
			}

			if resp.StatusCode != http.StatusAccepted {
				results <- fmt.Errorf("StatusCode check failed on %v", resp.StatusCode)
				return
			}

			results <- nil
		}()
	}
	close(latch)
	for i := 0; i < concurrentFuncs; i++ {
		err := <-results
		if err != nil {
			t.Fatalf("Error in basic concurrency execution test: %v", err)
		}
	}
}
