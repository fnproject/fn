// IronCache (cloud k/v store) client library
package cache

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"github.com/iron-io/iron_go3/api"
	"github.com/iron-io/iron_go3/config"
)

var (
	JSON = Codec{Marshal: json.Marshal, Unmarshal: json.Unmarshal}
	Gob  = Codec{Marshal: gobMarshal, Unmarshal: gobUnmarshal}
)

type Cache struct {
	Settings config.Settings
	Name     string
}

type Item struct {
	// Value is the Item's value
	Value interface{}
	// Object is the Item's value for use with a Codec.
	Object interface{}
	// Number of seconds until expiration. The zero value defaults to 7 days,
	// maximum is 30 days.
	Expiration time.Duration
	// Caches item only if the key is currently cached.
	Replace bool
	// Caches item only if the key isn't currently cached.
	Add bool
}

// New returns a struct ready to make requests with.
// The cacheName argument is used as namespace.
func New(cacheName string) *Cache {
	return &Cache{Settings: config.Config("iron_cache"), Name: cacheName}
}

func (c *Cache) caches(suffix ...string) *api.URL {
	return api.Action(c.Settings, "caches", suffix...)
}

func (c *Cache) ListCaches(page, perPage int) (caches []*Cache, err error) {
	out := []struct {
		Project_id string
		Name       string
	}{}

	err = c.caches().
		QueryAdd("page", "%d", page).
		QueryAdd("per_page", "%d", perPage).
		Req("GET", nil, &out)
	if err != nil {
		return
	}

	caches = make([]*Cache, 0, len(out))
	for _, item := range out {
		caches = append(caches, &Cache{
			Settings: c.Settings,
			Name:     item.Name,
		})
	}

	return
}

func (c *Cache) ServerVersion() (version string, err error) {
	out := map[string]string{}
	err = api.VersionAction(c.Settings).Req("GET", nil, &out)
	if err != nil {
		return
	}
	return out["version"], nil
}

func (c *Cache) Clear() (err error) {
	return c.caches(c.Name, "clear").Req("POST", nil, nil)
}

// Put adds an Item to the cache, overwriting any existing key of the same name.
func (c *Cache) Put(key string, item *Item) (err error) {
	in := struct {
		Value     interface{} `json:"value"`
		ExpiresIn int         `json:"expires_in,omitempty"`
		Replace   bool        `json:"replace,omitempty"`
		Add       bool        `json:"add,omitempty"`
	}{
		Value:     item.Value,
		ExpiresIn: int(item.Expiration.Seconds()),
		Replace:   item.Replace,
		Add:       item.Add,
	}

	return c.caches(c.Name, "items", key).Req("PUT", &in, nil)
}

func anyToString(value interface{}) (str interface{}, err error) {
	switch v := value.(type) {
	case string:
		str = v
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64:
		str = v
	case float32, float64:
		str = v
	case bool:
		str = v
	case fmt.Stringer:
		str = v.String()
	default:
		var bytes []byte
		if bytes, err = json.Marshal(value); err == nil {
			str = string(bytes)
		}
	}

	return
}

func (c *Cache) Set(key string, value interface{}, ttl ...int) (err error) {
	str, err := anyToString(value)
	if err == nil {
		if len(ttl) > 0 {
			err = c.Put(key, &Item{Value: str, Expiration: time.Duration(ttl[0]) * time.Second})
		} else {
			err = c.Put(key, &Item{Value: str})
		}
	}
	return
}
func (c *Cache) Add(key string, value ...interface{}) (err error) {
	str, err := anyToString(value)
	if err == nil {
		err = c.Put(key, &Item{
			Value: str, Expiration: time.Duration(123) * time.Second, Add: true,
		})
	}
	return
}
func (c *Cache) Replace(key string, value ...interface{}) (err error) {
	str, err := anyToString(value)
	if err == nil {
		err = c.Put(key, &Item{
			Value: str, Expiration: time.Duration(123) * time.Second, Replace: true,
		})
	}
	return
}

// Increment increments the corresponding item's value.
func (c *Cache) Increment(key string, amount int64) (value interface{}, err error) {
	in := map[string]int64{"amount": amount}

	out := struct {
		Message string      `json:"msg"`
		Value   interface{} `json:"value"`
	}{}
	if err = c.caches(c.Name, "items", key, "increment").Req("POST", &in, &out); err == nil {
		value = out.Value
	}
	return
}

// Get gets an item from the cache.
func (c *Cache) Get(key string) (value interface{}, err error) {
	out := struct {
		Cache string      `json:"cache"`
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}{}
	if err = c.caches(c.Name, "items", key).Req("GET", nil, &out); err == nil {
		value = out.Value
	}
	return
}

func (c *Cache) GetMeta(key string) (value map[string]interface{}, err error) {
	value = map[string]interface{}{}
	err = c.caches(c.Name, "items", key).Req("GET", nil, &value)
	return
}

// Delete removes an item from the cache.
func (c *Cache) Delete(key string) (err error) {
	return c.caches(c.Name, "items", key).Req("DELETE", nil, nil)
}

type Codec struct {
	Marshal   func(interface{}) ([]byte, error)
	Unmarshal func([]byte, interface{}) error
}

func (cd Codec) Put(c *Cache, key string, item *Item) (err error) {
	bytes, err := cd.Marshal(item.Object)
	if err != nil {
		return
	}

	item.Value = string(bytes)

	return c.Put(key, item)
}

func (cd Codec) Get(c *Cache, key string, object interface{}) (err error) {
	value, err := c.Get(key)
	if err != nil {
		return
	}

	err = cd.Unmarshal([]byte(value.(string)), object)
	if err != nil {
		return
	}

	return
}

func gobMarshal(v interface{}) ([]byte, error) {
	writer := bytes.Buffer{}
	enc := gob.NewEncoder(&writer)
	err := enc.Encode(v)
	return writer.Bytes(), err
}

func gobUnmarshal(marshalled []byte, v interface{}) error {
	reader := bytes.NewBuffer(marshalled)
	dec := gob.NewDecoder(reader)
	return dec.Decode(v)
}
