/*
Package kafka implements a Kafka reporter to send spans to a Kafka server/cluster.
*/
package kafka

import (
	"encoding/json"
	"log"
	"os"

	"github.com/Shopify/sarama"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/reporter"
)

// defaultKafkaTopic sets the standard Kafka topic our Reporter will publish
// on. The default topic for zipkin-receiver-kafka is "zipkin", see:
// https://github.com/openzipkin/zipkin/tree/master/zipkin-receiver-kafka
const defaultKafkaTopic = "zipkin"

// kafkaReporter implements Reporter by publishing spans to a Kafka
// broker.
type kafkaReporter struct {
	producer sarama.AsyncProducer
	logger   *log.Logger
	topic    string
}

// ReporterOption sets a parameter for the kafkaReporter
type ReporterOption func(c *kafkaReporter)

// Logger sets the logger used to report errors in the collection
// process.
func Logger(logger *log.Logger) ReporterOption {
	return func(c *kafkaReporter) {
		c.logger = logger
	}
}

// Producer sets the producer used to produce to Kafka.
func Producer(p sarama.AsyncProducer) ReporterOption {
	return func(c *kafkaReporter) {
		c.producer = p
	}
}

// Topic sets the kafka topic to attach the reporter producer on.
func Topic(t string) ReporterOption {
	return func(c *kafkaReporter) {
		c.topic = t
	}
}

// NewReporter returns a new Kafka-backed Reporter. address should be a slice of
// TCP endpoints of the form "host:port".
func NewReporter(address []string, options ...ReporterOption) (reporter.Reporter, error) {
	c := &kafkaReporter{
		logger: log.New(os.Stderr, "", log.LstdFlags),
		topic:  defaultKafkaTopic,
	}

	for _, option := range options {
		option(c)
	}
	if c.producer == nil {
		p, err := sarama.NewAsyncProducer(address, nil)
		if err != nil {
			return nil, err
		}
		c.producer = p
	}

	go c.logErrors()

	return c, nil
}

func (c *kafkaReporter) logErrors() {
	for pe := range c.producer.Errors() {
		c.logger.Print("msg", pe.Msg, "err", pe.Err, "result", "failed to produce msg")
	}
}

func (c *kafkaReporter) Send(s model.SpanModel) {
	m, err := json.Marshal(s)
	if err != nil {
		c.logger.Printf("failed when marshalling the span: %s\n", err.Error())
		return
	}

	c.producer.Input() <- &sarama.ProducerMessage{
		Topic: c.topic,
		Key:   nil,
		Value: sarama.ByteEncoder(m),
	}
}

func (c *kafkaReporter) Close() error {
	return c.producer.Close()
}
