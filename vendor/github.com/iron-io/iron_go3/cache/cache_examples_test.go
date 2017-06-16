package cache_test

import (
	"fmt"
	"github.com/iron-io/iron_go3/cache"
)

func p(a ...interface{}) { fmt.Println(a...) }

func Example1StoringData() {
	// For configuration info, see http://dev.iron.io/articles/configuration
	c := cache.New("test_cache")

	// Numbers will get stored as numbers
	c.Set("number_item", 42)

	// Strings get stored as strings
	c.Set("string_item", "Hello, IronCache")

	// Objects and dicts get JSON-encoded and stored as strings
	c.Set("complex_item", map[string]interface{}{
		"test": "this is a dict",
		"args": []string{"apples", "oranges"},
	})

	p("all stored")
	// Output:
	// all stored
}

func Example2Incrementing() {
	c := cache.New("test_cache")

	p(c.Increment("number_item", 10))
	p(c.Get("number_item"))

	p(c.Increment("string_item", 10))

	p(c.Increment("complex_item", 10))

	// Output:
	// <nil>
	// 52 <nil>
	// 400 Bad Request: Cannot increment or decrement non-numeric value
	// 400 Bad Request: Cannot increment or decrement non-numeric value
}

func Example3Decrementing() {
	c := cache.New("test_cache")

	p(c.Increment("number_item", -10))
	p(c.Get("number_item"))

	p(c.Increment("string_item", -10))

	p(c.Increment("complex_item", -10))

	// Output:
	// <nil>
	// 42 <nil>
	// 400 Bad Request: Cannot increment or decrement non-numeric value
	// 400 Bad Request: Cannot increment or decrement non-numeric value
}

func Example4RetrievingData() {
	c := cache.New("test_cache")

	value, err := c.Get("number_item")
	fmt.Printf("%#v (%#v)\n", value, err)

	value, err = c.Get("string_item")
	fmt.Printf("%#v (%#v)\n", value, err)

	// JSON is returned as strings
	value, err = c.Get("complex_item")
	fmt.Printf("%#v (%#v)\n", value, err)

	// You can use the JSON codec to deserialize it.
	obj := struct {
		Args []string
		Test string
	}{}
	err = cache.JSON.Get(c, "complex_item", &obj)
	fmt.Printf("%#v (%#v)\n", obj, err)
	// Output:
	// 42 (<nil>)
	// "Hello, IronCache" (<nil>)
	// "{\"args\":[\"apples\",\"oranges\"],\"test\":\"this is a dict\"}" (<nil>)
	// struct { Args []string; Test string }{Args:[]string{"apples", "oranges"}, Test:"this is a dict"} (<nil>)
}

func Example5DeletingData() {
	c := cache.New("test_cache")

	// Immediately delete an item
	c.Delete("string_item")

	p(c.Get("string_item"))
	// Output:
	// <nil> 404 Not Found: The resource, project, or endpoint being requested doesn't exist.
}
