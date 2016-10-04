package mqs

import (
	"github.com/iron-io/functions/api/models"
	"golang.org/x/net/context"
)

type Mock struct {
	FakeApp    *models.App
	FakeApps   []*models.App
	FakeRoute  *models.Route
	FakeRoutes []*models.Route
}

func (mock *Mock) Push(context.Context, *models.Task) (*models.Task, error) {
	return nil, nil
}

func (mock *Mock) Reserve(context.Context) (*models.Task, error) {
	return nil, nil
}

func (mock *Mock) Delete(context.Context, *models.Task) error {
	return nil
}
