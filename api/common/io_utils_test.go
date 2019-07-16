package common

import (
	"errors"
	"reflect"
	"testing"
)

var overflowError = errors.New("overflow")
var writerError = errors.New("writeFailed")

type writeSpec struct {
	write       []byte
	writerGot   []byte
	writerErr   error
	expectBytes int
	expectErr   error
	writerWrote int
}

type specWriter struct {
	t    *testing.T
	spec []writeSpec
	idx  int
}

func (sw *specWriter) Write(bytes []byte) (int, error) {
	if sw.idx >= len(sw.spec) {
		sw.t.Fatalf("Too many operations against spec")
	}

	spec := sw.spec[sw.idx]
	if spec.writerGot == nil {
		sw.t.Fatalf("Unexpected write to writer ")
	}
	sw.idx++
	if !reflect.DeepEqual(bytes, spec.writerGot) {
		sw.t.Fatalf("Unexpected write :  got %v expected %v", bytes, spec.writerGot)
	}
	return spec.writerWrote, spec.writerErr

}

func TestClamWriter(t *testing.T) {
	stillOverflows := writeSpec{
		write:       []byte("blah"),
		writerGot:   nil,
		writerErr:   nil,
		expectBytes: 0,
		expectErr:   overflowError,
		writerWrote: 0,
	}
	cases := []struct {
		name   string
		max    uint64
		writes []writeSpec
	}{
		{"zero limit,empty write",
			uint64(0),
			[]writeSpec{{
				[]byte{},
				[]byte{},
				nil,
				0,
				nil,
				0,
			}},
		},
		{"zero limit one byte write",
			uint64(0),
			[]writeSpec{{
				[]byte("a"),
				[]byte("a"),
				nil,
				1,
				nil,
				1,
			}},
		},
		{"one limit one byte write",
			uint64(1),
			[]writeSpec{{
				[]byte("a"),
				[]byte("a"),
				nil,
				1,
				nil,
				1,
			}},
		},
		{"one limit two one byte writes",
			uint64(1),
			[]writeSpec{
				{
					[]byte("a"),
					[]byte("a"),
					nil,
					1,
					nil,
					1,
				},
				stillOverflows,
				stillOverflows,
			},
		},
		{"one limit one two-byte write",
			uint64(1),
			[]writeSpec{{
				[]byte("ab"),
				[]byte("a"),
				nil,
				1,
				overflowError,
				1,
			}, stillOverflows},
		},

		{"three limit;  two, two byte writes",
			uint64(3),
			[]writeSpec{
				{
					[]byte("ab"),
					[]byte("ab"),
					nil,
					2,
					nil,
					2,
				},
				{
					[]byte("ab"),
					[]byte("a"),
					nil,
					1,
					overflowError,
					1,
				},
				stillOverflows,
			},
		},

		{"partial-write ok ",
			uint64(3),
			[]writeSpec{
				{
					[]byte("abc"),
					[]byte("abc"),
					writerError,
					2,
					writerError,
					2,
				},
				{
					[]byte("a"),
					[]byte("a"),
					nil,
					1,
					nil,
					1,
				},
				stillOverflows,
			},
		},

		{"partial-write overflow ",
			uint64(3),
			[]writeSpec{
				{
					[]byte("abc"),
					[]byte("abc"),
					writerError,
					2,
					writerError,
					2,
				},
				{
					[]byte("ab"),
					[]byte("a"),
					nil,
					1,
					overflowError,
					1,
				},
				stillOverflows,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specWriter := &specWriter{
				t:    t,
				spec: tc.writes,
			}

			cw := NewClampWriter(specWriter, tc.max, overflowError)

			for _, write := range tc.writes {
				written, err := cw.Write(write.write)
				if write.expectBytes != written {
					t.Fatalf("Unexpected write returned, got %d, wanted %d", written, write.expectBytes)
				}
				if write.expectErr != err {
					t.Fatalf("Unexpected err got %v, wanted %v", err, write.expectErr)
				}
			}

		})
	}

}
