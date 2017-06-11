package statsdtest

import (
	"bytes"
	"reflect"
	"testing"
)

type parsingTestCase struct {
	name     string
	sent     [][]byte
	expected Stats
}

var (
	badStatNameOnly = []byte("foo.bar.baz:")
	bsnoStat        = Stat{
		Raw:    badStatNameOnly,
		Stat:   "foo.bar.baz",
		Parsed: false,
	}

	gaugeWithoutRate = []byte("foo.bar.baz:1.000|g")
	gworStat         = Stat{
		Raw:    gaugeWithoutRate,
		Stat:   "foo.bar.baz",
		Value:  "1.000",
		Tag:    "g",
		Parsed: true,
	}

	counterWithRate = []byte("foo.bar.baz:1.000|c|@0.75")
	cwrStat         = Stat{
		Raw:    counterWithRate,
		Stat:   "foo.bar.baz",
		Value:  "1.000",
		Tag:    "c",
		Rate:   "0.75",
		Parsed: true,
	}

	stringStat = []byte(":some string value|s")
	sStat      = Stat{
		Raw:    stringStat,
		Stat:   "",
		Value:  "some string value",
		Tag:    "s",
		Parsed: true,
	}

	badValue = []byte("asoentuh")
	bvStat   = Stat{Raw: badValue}

	testCases = []parsingTestCase{
		{name: "no stat data",
			sent:     [][]byte{badStatNameOnly},
			expected: Stats{bsnoStat}},
		{name: "trivial case",
			sent:     [][]byte{gaugeWithoutRate},
			expected: Stats{gworStat}},
		{name: "multiple simple",
			sent:     [][]byte{gaugeWithoutRate, counterWithRate},
			expected: Stats{gworStat, cwrStat}},
		{name: "mixed good and bad",
			sent:     [][]byte{badValue, badValue, stringStat, badValue, counterWithRate, badValue},
			expected: Stats{bvStat, bvStat, sStat, bvStat, cwrStat, bvStat}},
	}
)

func TestParseBytes(t *testing.T) {
	for _, tc := range testCases {
		got := ParseStats(bytes.Join(tc.sent, []byte("\n")))
		want := tc.expected
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got: %+v, want: %+v", tc.name, got, want)
		}
	}
}

func TestStatsUnparsed(t *testing.T) {
	start := Stats{bsnoStat, gworStat, bsnoStat, bsnoStat, cwrStat}
	got := start.Unparsed()
	want := Stats{bsnoStat, bsnoStat, bsnoStat}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %+v, want: %+v", got, want)
	}
}

func TestStatsCollectNamed(t *testing.T) {
	type test struct {
		name    string
		start   Stats
		want    Stats
		matchOn string
	}

	cases := []test{
		{"No matches",
			Stats{bsnoStat, cwrStat},
			nil,
			"foo"},
		{"One match",
			Stats{bsnoStat, Stat{Stat: "foo"}, cwrStat},
			Stats{Stat{Stat: "foo"}},
			"foo"},
		{"Two matches",
			Stats{bsnoStat, Stat{Stat: "foo"}, cwrStat},
			Stats{bsnoStat, cwrStat},
			"foo.bar.baz"},
	}

	for _, c := range cases {
		got := c.start.CollectNamed(c.matchOn)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got: %+v, want: %+v", c.name, got, c.want)
		}
	}
}

func TestStatsCollect(t *testing.T) {
	type test struct {
		name  string
		start Stats
		want  Stats
		pred  func(Stat) bool
	}

	cases := []test{
		{"Not called",
			Stats{},
			nil,
			func(_ Stat) bool { t.Errorf("should not be called"); return true }},
		{"Matches value = 1.000",
			Stats{bsnoStat, gworStat, cwrStat, sStat, bsnoStat},
			Stats{gworStat, cwrStat},
			func(s Stat) bool { return s.Value == "1.000" }},
	}

	for _, c := range cases {
		got := c.start.Collect(c.pred)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got: %+v, want: %+v", c.name, got, c.want)
		}
	}
}

func TestStatsValues(t *testing.T) {
	start := Stats{bsnoStat, sStat, gworStat}
	got := start.Values()
	want := []string{bsnoStat.Value, sStat.Value, gworStat.Value}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %+v, want: %+v", got, want)
	}
}
