package datastore

import (
	"testing"

	"github.com/fnproject/fn/api/datastore/datastoretest"
	"github.com/fnproject/fn/api/models"
)

func TestDatastore(t *testing.T) {
	f := func(t *testing.T) models.Datastore {
		return NewMock()
	}
	datastoretest.RunAllTests(t, f, datastoretest.NewBasicResourceProvider())
}
