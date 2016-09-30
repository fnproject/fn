package mqs

import "github.com/iron-io/functions/api/models"

type Mock struct {
	FakeApp    *models.App
	FakeApps   []*models.App
	FakeRoute  *models.Route
	FakeRoutes []*models.Route
}

func (mock *Mock) Push(*models.Task) (*models.Task, error) {
	return nil, nil
}

func (mock *Mock) Reserve() (*models.Task, error) {
	return nil, nil
}

func (mock *Mock) Delete(*models.Task) error {
	return nil
}
