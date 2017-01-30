package server

import "testing"

type testSpecialHandler struct{}

func (h *testSpecialHandler) Handle(c HandlerContext) error {
	// c.Set(api.AppName, "test")
	return nil
}

func TestSpecialHandlerSet(t *testing.T) {
	// todo: temporarily commented as we may remove special handlers
	// ctx := context.Background()

	// tasks := make(chan task.Request)
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// rnr, cancelrnr := testRunner(t)
	// defer cancelrnr()

	// go runner.StartWorkers(ctx, rnr, tasks)

	// s := &Server{
	// 	Runner: rnr,
	// 	Router: gin.New(),
	// 	Datastore: &datastore.Mock{
	// 		Apps: []*models.App{
	// 			{Name: "test"},
	// 		},
	// 		Routes: []*models.Route{
	// 			{Path: "/test", Image: "iron/hello", AppName: "test"},
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
