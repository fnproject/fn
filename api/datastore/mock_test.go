package datastore

import (
	"testing"

	"gitlab-odx.oracle.com/odx/functions/api/datastore/internal/datastoretest"
)

func TestDatastore(t *testing.T) {
	datastoretest.Test(t, NewMock())
}