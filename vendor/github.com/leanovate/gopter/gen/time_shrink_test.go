package gen_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/leanovate/gopter/gen"
)

func TestTimeShrink(t *testing.T) {
	timeShrink := gen.TimeShrinker(time.Unix(20, 10)).All()
	if !reflect.DeepEqual(timeShrink, []interface{}{
		time.Unix(0, 10),
		time.Unix(20, 0),
		time.Unix(10, 10),
		time.Unix(20, 5),
		time.Unix(15, 10),
		time.Unix(20, 8),
		time.Unix(18, 10),
		time.Unix(20, 9),
		time.Unix(19, 10),
	}) {
		t.Errorf("Invalid timeShrink: %#v", timeShrink)
	}

}
