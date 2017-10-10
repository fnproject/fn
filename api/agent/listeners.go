package agent

import (
	"context"

	"github.com/fnproject/fn/api/extenders"
	"github.com/fnproject/fn/api/models"
)

func (a *agent) AddCallListener(listener extenders.CallListener) {
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
