package kafka_test

import (
	"errors"
	"testing"
	"time"

	"encoding/json"
	"log"

	"github.com/Shopify/sarama"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/reporter"
	"github.com/openzipkin/zipkin-go/reporter/kafka"
)

type stubProducer struct {
	in        chan *sarama.ProducerMessage
	err       chan *sarama.ProducerError
	kafkaDown bool
	closed    bool
}

func (p *stubProducer) AsyncClose() {}
func (p *stubProducer) Close() error {
	if p.kafkaDown {
		return errors.New("kafka is down")
	}
	p.closed = true
	return nil
}
func (p *stubProducer) Input() chan<- *sarama.ProducerMessage     { return p.in }
func (p *stubProducer) Successes() <-chan *sarama.ProducerMessage { return nil }
func (p *stubProducer) Errors() <-chan *sarama.ProducerError      { return p.err }

func newStubProducer(kafkaDown bool) *stubProducer {
	return &stubProducer{
		make(chan *sarama.ProducerMessage),
		make(chan *sarama.ProducerError),
		kafkaDown,
		false,
	}
}

var spans = []*model.SpanModel{
	makeNewSpan("avg", 123, 456, 0, true),
	makeNewSpan("sum", 123, 789, 456, true),
	makeNewSpan("div", 123, 101112, 456, true),
}

func TestKafkaProduce(t *testing.T) {
	p := newStubProducer(false)
	c, err := kafka.NewReporter(
		[]string{"192.0.2.10:9092"}, kafka.Producer(p),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range spans {
		m := sendSpan(t, c, p, *want)
		testMetadata(t, m)
		got := deserializeSpan(t, m.Value)
		testEqual(t, want, got)
	}
}

func TestKafkaClose(t *testing.T) {
	p := newStubProducer(false)
	c, err := kafka.NewReporter(
		[]string{"192.0.2.10:9092"}, kafka.Producer(p),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if !p.closed {
		t.Fatal("producer not closed")
	}
}

func TestKafkaCloseError(t *testing.T) {
	p := newStubProducer(true)
	c, err := kafka.NewReporter(
		[]string{"192.0.2.10:9092"}, kafka.Producer(p),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err == nil {
		t.Error("no error on close")
	}
}

type chanWriter struct {
	errs chan []interface{}
}

func (cw *chanWriter) Write(p []byte) (n int, err error) {
	cw.errs <- []interface{}{p}

	return 1, nil
}

func TestKafkaErrors(t *testing.T) {
	p := newStubProducer(true)
	errs := make(chan []interface{}, len(spans))

	c, err := kafka.NewReporter(
		[]string{"192.0.2.10:9092"},
		kafka.Producer(p),
		kafka.Logger(log.New(&chanWriter{errs}, "", log.LstdFlags)),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range spans {
		_ = sendSpan(t, c, p, *want)
	}

	for i := 0; i < len(spans); i++ {
		select {
		case <-errs:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("errors not logged. got %d, wanted %d", i, len(spans))
		}
	}
}

func sendSpan(t *testing.T, c reporter.Reporter, p *stubProducer, s model.SpanModel) *sarama.ProducerMessage {
	var m *sarama.ProducerMessage
	rcvd := make(chan bool, 1)
	go func() {
		select {
		case m = <-p.in:
			rcvd <- true
			if p.kafkaDown {
				p.err <- &sarama.ProducerError{
					Msg: m,
					Err: errors.New("kafka is down"),
				}
			}
		case <-time.After(100 * time.Millisecond):
			rcvd <- false
		}
	}()

	c.Send(s)

	if !<-rcvd {
		t.Fatal("span message was not produced")
	}
	return m
}

func testMetadata(t *testing.T, m *sarama.ProducerMessage) {
	if m.Topic != "zipkin" {
		t.Errorf("produced to topic %q, want %q", m.Topic, "zipkin")
	}
	if m.Key != nil {
		t.Errorf("produced with key %q, want nil", m.Key)
	}
}

func deserializeSpan(t *testing.T, e sarama.Encoder) *model.SpanModel {
	bytes, err := e.Encode()
	if err != nil {
		t.Errorf("error in encoding: %v", err)
	}

	var s model.SpanModel

	err = json.Unmarshal(bytes, &s)
	if err != nil {
		t.Errorf("error in decoding: %v", err)
		return nil
	}

	return &s
}

func testEqual(t *testing.T, want *model.SpanModel, got *model.SpanModel) {
	if got.TraceID != want.TraceID {
		t.Errorf("trace_id %d, want %d", got.TraceID, want.TraceID)
	}
	if got.ID != want.ID {
		t.Errorf("id %d, want %d", got.ID, want.ID)
	}
	if got.ParentID == nil {
		if want.ParentID != nil {
			t.Errorf("parent_id %d, want %d", got.ParentID, want.ParentID)
		}
	} else if *got.ParentID != *want.ParentID {
		t.Errorf("parent_id %d, want %d", got.ParentID, want.ParentID)
	}
}

func makeNewSpan(methodName string, traceID, spanID, parentSpanID uint64, debug bool) *model.SpanModel {
	timestamp := time.Now()

	var parentID = new(model.ID)
	if parentSpanID != 0 {
		*parentID = model.ID(parentSpanID)
	}

	return &model.SpanModel{
		SpanContext: model.SpanContext{
			TraceID:  model.TraceID{Low: traceID},
			ID:       model.ID(spanID),
			ParentID: parentID,
			Debug:    debug,
		},
		Name:      methodName,
		Timestamp: timestamp,
	}
}
