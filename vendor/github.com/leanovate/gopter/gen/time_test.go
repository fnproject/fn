package gen_test

import (
	"testing"
	"time"

	"github.com/leanovate/gopter/gen"
)

func TestTime(t *testing.T) {
	timeGen := gen.Time()
	for i := 0; i < 100; i++ {
		value, ok := timeGen.Sample()

		if !ok || value == nil {
			t.Errorf("Invalid time: %#v", value)
		}
		v, ok := value.(time.Time)
		if !ok || v.String() == "" {
			t.Errorf("Invalid time: %#v", value)
		}
		if v.Year() < 0 || v.Year() > 9999 {
			t.Errorf("Year out of range: %#v", v)
		}
	}
}

func TestAnyTime(t *testing.T) {
	timeGen := gen.AnyTime()
	for i := 0; i < 100; i++ {
		value, ok := timeGen.Sample()

		if !ok || value == nil {
			t.Errorf("Invalid time: %#v", value)
		}
		v, ok := value.(time.Time)
		if !ok || v.String() == "" {
			t.Errorf("Invalid time: %#v", value)
		}
	}
}

func TestTimeRegion(t *testing.T) {
	duration := time.Duration(10*24*365) * time.Hour
	from := time.Unix(1000, 0)
	until := from.Add(duration)
	timeRange := gen.TimeRange(from, duration)

	for i := 0; i < 100; i++ {
		value, ok := timeRange.Sample()

		if !ok || value == nil {
			t.Errorf("Invalid time: %#v", value)
		}
		v, ok := value.(time.Time)
		if !ok || v.Before(from) || v.After(until) {
			t.Errorf("Invalid time: %#v", value)
		}
	}
}
