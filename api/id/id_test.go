package id

import (
	"encoding/binary"
	"math"
	"net"
	"testing"
	"time"
)

func BenchmarkGen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		id := New()
		_ = id
	}
}

func BenchmarkMarshalText(b *testing.B) {
	id := New()
	for i := 0; i < b.N; i++ {
		byts, _ := id.MarshalText()
		_ = byts
	}
}

func BenchmarkUnmarshalText(b *testing.B) {
	id := New()
	byts, _ := id.MarshalText()
	for i := 0; i < b.N; i++ {
		var id Id
		id.UnmarshalText(byts)
		_ = id
	}
}

func BenchmarkValidateText(b *testing.B) {
	id := New()
	byts, _ := id.MarshalText()
	for i := 0; i < b.N; i++ {
		ValidateText(byts)
	}
}

func TestValidInValid(t *testing.T) {
	id := New()
	byts, _ := id.MarshalText()
	if !ValidateText(byts) {
		t.Fatal("valid id should pass")
	}
	byts[5] = ' '
	if ValidateText(byts) {
		t.Fatal("invalid id should not pass")
	}
}

func TestIdRaw(t *testing.T) {
	SetMachineIdHost(net.IP{127, 0, 0, 1}, 8080)

	ts := time.Now()
	ms := uint64(ts.Unix())*1000 + uint64(ts.Nanosecond()/int(time.Millisecond))
	count := uint32(math.MaxUint32)
	id := newID(ms, machineID, count)

	var buf [8]byte
	copy(buf[2:], id[:6])
	idTime := binary.BigEndian.Uint64(buf[:])
	if ms != idTime {
		t.Fatal("id time doesn't not match time given", ms, idTime)
	}

	copy(buf[2:], id[6:12])
	idMid := binary.BigEndian.Uint64(buf[:])
	if idMid != machineID {
		t.Fatal("machine id mismatch", idMid, machineID)
	}

	idCount := binary.BigEndian.Uint32(id[12:16])
	if idCount != count {
		t.Fatal("count mismatch", idCount, count)
	}
}

func TestDescending(t *testing.T) {
	id := "0123WXYZ"

	flip := EncodeDescending(id)

	if len(flip) != len(id) {
		t.Fatal("flipped string has different length:", len(flip), len(id))
	}

	for i := range flip {
		if flip[i] != id[len(id)-1-i] {
			t.Fatalf("flipped encoding not working. got: %v, want: %v", flip[i], id[len(id)-1-i])
		}
	}
}
