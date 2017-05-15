package datastore

import (
	"testing"

	"github.com/treeder/functions/api/datastore/internal/datastoretest"
)

func TestDatastore(t *testing.T) {
	datastoretest.Test(t, NewMock())
}