package mqs

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/opentracing/opentracing-go"
	"github.com/fnproject/fn/api/models"
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
	if strings.HasPrefix(u.Scheme, "ironmq") {
		return NewIronMQ(u), nil
	}

	return nil, fmt.Errorf("mq type not supported %v", u.Scheme)
}

type metricMQ struct {
	mq models.MessageQueue
}

func (m *metricMQ) Push(ctx context.Context, t *models.Task) (*models.Task, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "mq_push")
	defer span.Finish()
	return m.mq.Push(ctx, t)
}

func (m *metricMQ) Reserve(ctx context.Context) (*models.Task, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "mq_reserve")
	defer span.Finish()
	return m.mq.Reserve(ctx)
}

func (m *metricMQ) Delete(ctx context.Context, t *models.Task) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "mq_delete")
	defer span.Finish()
	return m.mq.Delete(ctx, t)
}
