package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestFloat64Shrinker(t *testing.T) {
	zeroShrinks := gen.Float64Shrinker(float64(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	oneShrinks := gen.Float64Shrinker(float64(1)).All()
	if !reflect.DeepEqual(oneShrinks, []interface{}{
		0.0,
		0.5,
		-0.5,
		0.75,
		-0.75,
		0.875,
		-0.875,
		0.9375,
		-0.9375,
		0.96875,
		-0.96875,
		0.984375,
		-0.984375,
		0.9921875,
		-0.9921875,
		0.99609375,
		-0.99609375,
		0.998046875,
		-0.998046875,
		0.9990234375,
		-0.9990234375,
		0.99951171875,
		-0.99951171875,
		0.999755859375,
		-0.999755859375,
		0.9998779296875,
		-0.9998779296875,
		0.99993896484375,
		-0.99993896484375,
		0.999969482421875,
		-0.999969482421875,
		0.9999847412109375,
		-0.9999847412109375,
	}) {
		t.Errorf("Invalid tenShrinks: %#v", oneShrinks)
	}

	hundretShrinks := gen.Float64Shrinker(float64(100)).All()
	if !reflect.DeepEqual(hundretShrinks, []interface{}{
		0.0,
		50.0,
		-50.0,
		75.0,
		-75.0,
		87.5,
		-87.5,
		93.75,
		-93.75,
		96.875,
		-96.875,
		98.4375,
		-98.4375,
		99.21875,
		-99.21875,
		99.609375,
		-99.609375,
		99.8046875,
		-99.8046875,
		99.90234375,
		-99.90234375,
		99.951171875,
		-99.951171875,
		99.9755859375,
		-99.9755859375,
		99.98779296875,
		-99.98779296875,
		99.993896484375,
		-99.993896484375,
		99.9969482421875,
		-99.9969482421875,
		99.99847412109375,
		-99.99847412109375,
		99.99923706054688,
		-99.99923706054688,
		99.99961853027344,
		-99.99961853027344,
		99.99980926513672,
		-99.99980926513672,
		99.99990463256836,
		-99.99990463256836,
		99.99995231628418,
		-99.99995231628418,
		99.99997615814209,
		-99.99997615814209,
		99.99998807907104,
		-99.99998807907104,
	}) {
		t.Errorf("Invalid hundretShrinks: %#v", hundretShrinks)
	}
}

func TestFloat32Shrinker(t *testing.T) {
	zeroShrinks := gen.Float32Shrinker(float32(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	oneShrinks := gen.Float32Shrinker(float32(1)).All()
	if !reflect.DeepEqual(oneShrinks, []interface{}{
		float32(0),
		float32(0.5),
		float32(-0.5),
		float32(0.75),
		float32(-0.75),
		float32(0.875),
		float32(-0.875),
		float32(0.9375),
		float32(-0.9375),
		float32(0.96875),
		float32(-0.96875),
		float32(0.984375),
		float32(-0.984375),
		float32(0.9921875),
		float32(-0.9921875),
		float32(0.99609375),
		float32(-0.99609375),
		float32(0.9980469),
		float32(-0.9980469),
		float32(0.99902344),
		float32(-0.99902344),
		float32(0.9995117),
		float32(-0.9995117),
		float32(0.99975586),
		float32(-0.99975586),
		float32(0.9998779),
		float32(-0.9998779),
		float32(0.99993896),
		float32(-0.99993896),
		float32(0.9999695),
		float32(-0.9999695),
		float32(0.99998474),
		float32(-0.99998474),
	}) {
		t.Errorf("Invalid tenShrinks: %#v", oneShrinks)
	}

}
