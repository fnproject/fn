package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFavoriteService_List(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/favorites/list.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"user_id": "113419064", "since_id": "101492475", "include_entities": "false"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"text": "Gophercon talks!"}, {"text": "Why gophers are so adorable"}]`)
	})

	client := NewClient(httpClient)
	tweets, _, err := client.Favorites.List(&FavoriteListParams{UserID: 113419064, SinceID: 101492475, IncludeEntities: Bool(false)})
	expected := []Tweet{Tweet{Text: "Gophercon talks!"}, Tweet{Text: "Why gophers are so adorable"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweets)
}

func TestFavoriteService_Create(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/favorites/create.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertPostForm(t, map[string]string{"id": "12345"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 581980947630845953, "text": "very informative tweet"}`)
	})

	client := NewClient(httpClient)
	params := &FavoriteCreateParams{ID: 12345}
	tweet, _, err := client.Favorites.Create(params)
	assert.Nil(t, err)
	expected := &Tweet{ID: 581980947630845953, Text: "very informative tweet"}
	assert.Equal(t, expected, tweet)
}

func TestFavoriteService_Destroy(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/favorites/destroy.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertPostForm(t, map[string]string{"id": "12345"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 581980947630845953, "text": "very unhappy tweet"}`)
	})

	client := NewClient(httpClient)
	params := &FavoriteDestroyParams{ID: 12345}
	tweet, _, err := client.Favorites.Destroy(params)
	assert.Nil(t, err)
	expected := &Tweet{ID: 581980947630845953, Text: "very unhappy tweet"}
	assert.Equal(t, expected, tweet)
}
