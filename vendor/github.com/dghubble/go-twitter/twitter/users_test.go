package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserService_Show(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/users/show.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"screen_name": "xkcdComic"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name": "XKCD Comic", "favourites_count": 2}`)
	})

	client := NewClient(httpClient)
	user, _, err := client.Users.Show(&UserShowParams{ScreenName: "xkcdComic"})
	expected := &User{Name: "XKCD Comic", FavouritesCount: 2}
	assert.Nil(t, err)
	assert.Equal(t, expected, user)
}

func TestUserService_LookupWithIds(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/users/lookup.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"user_id": "113419064,623265148"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"screen_name": "golang"}, {"screen_name": "dghubble"}]`)
	})

	client := NewClient(httpClient)
	users, _, err := client.Users.Lookup(&UserLookupParams{UserID: []int64{113419064, 623265148}})
	expected := []User{User{ScreenName: "golang"}, User{ScreenName: "dghubble"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, users)
}

func TestUserService_LookupWithScreenNames(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/users/lookup.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"screen_name": "foo,bar"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"name": "Foo"}, {"name": "Bar"}]`)
	})

	client := NewClient(httpClient)
	users, _, err := client.Users.Lookup(&UserLookupParams{ScreenName: []string{"foo", "bar"}})
	expected := []User{User{Name: "Foo"}, User{Name: "Bar"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, users)
}

func TestUserService_Search(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/users/search.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"count": "11", "q": "news"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"name": "BBC"}, {"name": "BBC Breaking News"}]`)
	})

	client := NewClient(httpClient)
	users, _, err := client.Users.Search("news", &UserSearchParams{Query: "override me", Count: 11})
	expected := []User{User{Name: "BBC"}, User{Name: "BBC Breaking News"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, users)
}

func TestUserService_SearchHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/users/search.json", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, map[string]string{"q": "news"}, r)
	})
	client := NewClient(httpClient)
	client.Users.Search("news", nil)
}
