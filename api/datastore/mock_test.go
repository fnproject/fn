package datastore

import (
	"testing"

	"github.com/fnproject/fn/api/datastore/internal/datastoretest"
)

func TestDatastore(t *testing.T) {
	datastoretest.Test(t, NewMock)
}
