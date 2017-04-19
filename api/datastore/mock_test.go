package datastore

import (
	"testing"

	"github.com/kumokit/functions/api/datastore/internal/datastoretest"
)

func TestDatastore(t *testing.T) {
	datastoretest.Test(t, NewMock())
}