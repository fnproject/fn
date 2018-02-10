package mqs

import (
	"context"
	"fmt"
	"net/url"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

// New will parse the URL and return the correct MQ implementation.
func New(mqURL string) (models.MessageQueue, error) {
	mq, err := newmq(mqURL)
	if err != nil {
		return nil, err
	}
	return &metricMQ{mq}, nil
}

func newmq(mqURL string) (models.MessageQueue, error) {
	// Play with URL schemes here: https://play.golang.org/p/xWAf9SpCBW
	u, err := url.Parse(mqURL)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": mqURL}).Fatal("bad MQ URL")
	}
	logrus.WithFields(logrus.Fields{"mq": u.Scheme}).Debug("selecting MQ")
	switch u.Scheme {
	case "memory":
		return NewMemoryMQ(), nil
	case "redis":
		return NewRedisMQ(u)
	case "bolt":
		return NewBoltMQ(u)
	}

	return nil, fmt.Errorf("mq type not supported %v", u.Scheme)
}

type metricMQ struct {
	mq models.MessageQueue
}

func (m *metricMQ) Push(ctx context.Context, t *models.Call) (*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "mq_push")
	defer span.End()
	return m.mq.Push(ctx, t)
}

func (m *metricMQ) Reserve(ctx context.Context) (*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "mq_reserve")
	defer span.End()
	return m.mq.Reserve(ctx)
}

func (m *metricMQ) Delete(ctx context.Context, t *models.Call) error {
	ctx, span := trace.StartSpan(ctx, "mq_delete")
	defer span.End()
	return m.mq.Delete(ctx, t)
}
