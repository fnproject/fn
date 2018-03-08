package id

import (
	"testing"
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
