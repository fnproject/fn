package server

import (
	"net/http"
	"testing"
)

type testSpecialHandler struct{}

func (h *testSpecialHandler) Handle(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	// r = r.WithContext(context.WithValue(r.Context(), api.AppName, "test"))
	return r, nil
}

func TestSpecialHandlerSet(t *testing.T) {
	// todo: temporarily commented as we may remove special handlers
	// ctx := context.Background()

	// tasks := make(chan task.Request)
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// rnr, cancelrnr := testRunner(t)
	// defer cancelrnr()

	// s := &Server{
	// 	Runner: rnr,
	// 	Router: gin.New(),
	// 	Datastore: &datastore.Mock{
	// 		Apps: []*models.App{
	// 			{Name: "test"},
	// 		},
	// 		Routes: []*models.Route{
	// 			{Path: "/test", Image: "funcy/hello", AppName: "test"},
	// 		},
	// 	},
	// 	MQ:      &mqs.Mock{},
	// 	tasks:   tasks,
	// 	Enqueue: DefaultEnqueue,
	// }

	// router := s.Router
	// router.Use(prepareMiddleware(ctx))
	// s.bindHandlers()
	// s.AddSpecialHandler(&testSpecialHandler{})

	// _, rec := routerRequest(t, router, "GET", "/test", nil)
	// if rec.Code != 200 {
	// 	dump, _ := httputil.DumpResponse(rec.Result(), true)
	// 	t.Fatalf("Test SpecialHandler: expected special handler to run functions successfully. Response:\n%s", dump)
	// }
}
