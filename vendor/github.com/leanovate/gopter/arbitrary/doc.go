/*
Package arbitrary contains helpers to create contexts of arbitrary values, i.e.
automatically combine generators as needed using reflection.

A simple example might look like this:

    func TestIntParse(t *testing.T) {
      properties := gopter.NewProperties(nil)
      arbitraries := arbitrary.DefaultArbitraries()

      properties.Property("printed integers can be parsed", arbitraries.ForAll(
    		func(a int64) bool {
    			str := fmt.Sprintf("%d", a)
    			parsed, err := strconv.ParseInt(str, 10, 64)
    			return err == nil && parsed == a
    		}))

      properties.TestingRun(t)
    }

Be aware that by default always the most generic generators are used. I.e. in
the example above the gen.Int64 generator will be used and the condition will
be tested for the full range of int64 numbers.

To adapt this one might register a generator for a specific type in an
arbitraries context. I.e. by adding

      arbitraries.RegisterGen(gen.Int64Range(-1000, 1000))

any generated int64 number will be between -1000 and 1000.
*/
package arbitrary
