package mqs

import (
	"context"

	"github.com/iron-io/functions/api/models"
)

type Mock struct {
	FakeApp   *models.App
	Apps      []*models.App
	FakeRoute *models.Route
	Routes    []*models.Route
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
