package gopter

import (
	"fmt"
	"reflect"
)

type derivedGen struct {
	biMapper   *BiMapper
	upGens     []Gen
	upSieves   []func(interface{}) bool
	upShrinker Shrinker
	resultType reflect.Type
}

func (d *derivedGen) Generate(genParams *GenParameters) *GenResult {
	labels := []string{}
	up := make([]interface{}, len(d.upGens))
	shrinkers := make([]Shrinker, len(d.upGens))
	sieves := make([]func(v interface{}) bool, len(d.upGens))

	var ok bool
	for i, gen := range d.upGens {
		result := gen(genParams)
		labels = append(labels, result.Labels...)
		shrinkers[i] = result.Shrinker
		sieves[i] = result.Sieve
		up[i], ok = result.Retrieve()
		if !ok {
			return &GenResult{
				Shrinker:   d.Shrinker,
				Result:     nil,
				Labels:     result.Labels,
				ResultType: d.resultType,
				Sieve:      d.Sieve,
			}
		}
	}
	down := d.biMapper.ConvertDown(up)
	if len(down) == 1 {
		return &GenResult{
			Shrinker:   d.Shrinker,
			Result:     down[0],
			Labels:     labels,
			ResultType: reflect.TypeOf(down[0]),
			Sieve:      d.Sieve,
		}
	}
	return &GenResult{
		Shrinker:   d.Shrinker,
		Result:     down,
		Labels:     labels,
		ResultType: reflect.TypeOf(down),
		Sieve:      d.Sieve,
	}
}

func (d *derivedGen) Sieve(down interface{}) bool {
	if down == nil {
		return false
	}
	downs, ok := down.([]interface{})
	if !ok {
		downs = []interface{}{down}
	}
	ups := d.biMapper.ConvertUp(downs)
	for i, up := range ups {
		if d.upSieves[i] != nil && !d.upSieves[i](up) {
			return false
		}
	}
	return true
}

func (d *derivedGen) Shrinker(down interface{}) Shrink {
	downs, ok := down.([]interface{})
	if !ok {
		downs = []interface{}{down}
	}
	ups := d.biMapper.ConvertUp(downs)
	upShrink := d.upShrinker(ups)

	return upShrink.Map(func(shrinkedUps []interface{}) interface{} {
		downs := d.biMapper.ConvertDown(shrinkedUps)
		if len(downs) == 1 {
			return downs[0]
		}
		return downs
	})
}

// DeriveGen derives a generator with shrinkers from a sequence of other
// generators mapped by a bijective function (BiMapper)
func DeriveGen(downstream interface{}, upstream interface{}, gens ...Gen) Gen {
	biMapper := NewBiMapper(downstream, upstream)

	if len(gens) != len(biMapper.UpTypes) {
		panic(fmt.Sprintf("Expected %d generators != %d", len(biMapper.UpTypes), len(gens)))
	}

	resultType := reflect.TypeOf([]interface{}{})
	if len(biMapper.DownTypes) == 1 {
		resultType = biMapper.DownTypes[0]
	}

	sieves := make([]func(interface{}) bool, len(gens))
	shrinkers := make([]Shrinker, len(gens))
	for i, gen := range gens {
		result := gen(DefaultGenParams)
		sieves[i] = result.Sieve
		shrinkers[i] = result.Shrinker
	}

	derived := &derivedGen{
		biMapper:   biMapper,
		upGens:     gens,
		upSieves:   sieves,
		upShrinker: CombineShrinker(shrinkers...),
		resultType: resultType,
	}
	return derived.Generate
}
