package agent

import (
	"context"

	"github.com/fnproject/fn/api/extensions"
	"github.com/fnproject/fn/api/models"
)

type callTrigger interface {
	fireBeforeCall(context.Context, *models.Call) error
	fireAfterCall(context.Context, *models.Call) error
}

func (a *agent) AddCallListener(listener extensions.CallListener) {
	a.callListeners = append(a.callListeners, listener)
}

func (a *agent) fireBeforeCall(ctx context.Context, call *models.Call) error {
	for _, l := range a.callListeners {
		err := l.BeforeCall(ctx, call)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *agent) fireAfterCall(ctx context.Context, call *models.Call) error {
	for _, l := range a.callListeners {
		err := l.AfterCall(ctx, call)
		if err != nil {
			return err
		}
	}
	return nil
}
