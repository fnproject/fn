package datastore

import (
	"testing"

	"gitlab.oracledx.com/odx/functions/api/datastore/internal/datastoretest"
)

func TestDatastore(t *testing.T) {
	datastoretest.Test(t, NewMock())
}