package supervisor

import (
	"context"
	"fmt"
	"sync"
)

var (
	universalAnonSvcMu sync.Mutex
	universalAnonSvc   uint64
)

func getUniversalAnonSvc() uint64 {
	universalAnonSvcMu.Lock()
	universalAnonSvc++
	v := universalAnonSvc
	universalAnonSvcMu.Unlock()
	return v
}

type anonymousService struct {
	id uint64
	f  func(context.Context)
}

func newAnonymousService(f func(context.Context)) *anonymousService {
	return &anonymousService{
		id: getUniversalAnonSvc(),
		f:  f,
	}
}

func (a anonymousService) Serve(ctx context.Context) {
	a.f(ctx)
}

func (a anonymousService) String() string {
	return fmt.Sprintf("anonymous service %d", a.id)
}
