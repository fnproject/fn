package agent

import (
	"context"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

type callTrigger interface {
	fireBeforeCall(context.Context, *models.Call) error
	fireAfterCall(context.Context, *models.Call) error
}

func (a *agent) AddCallListener(listener fnext.CallListener) {
	a.callListeners = append(a.callListeners, listener)
}

func (a *agent) fireBeforeCall(ctx context.Context, call *models.Call) error {
	return fireBeforeCallFun(a.callListeners, ctx, call)
}

func (a *agent) fireAfterCall(ctx context.Context, call *models.Call) error {
	return fireAfterCallFun(a.callListeners, ctx, call)
}

func fireBeforeCallFun(callListeners []fnext.CallListener, ctx context.Context, call *models.Call) error {
	for _, l := range callListeners {
		err := l.BeforeCall(ctx, call)
		if err != nil {
			return err
		}
	}
	return nil
}

func fireAfterCallFun(callListeners []fnext.CallListener, ctx context.Context, call *models.Call) error {
	for _, l := range callListeners {
		err := l.AfterCall(ctx, call)
		if err != nil {
			return err
		}
	}
	return nil
}
