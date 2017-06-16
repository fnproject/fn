package twitter

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusService_Show(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/show.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"id": "589488862814076930", "include_entities": "false"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"user": {"screen_name": "dghubble"}, "text": ".@audreyr use a DONTREADME file if you really want people to read it :P"}`)
	})

	client := NewClient(httpClient)
	params := &StatusShowParams{ID: 5441, IncludeEntities: Bool(false)}
	tweet, _, err := client.Statuses.Show(589488862814076930, params)
	expected := &Tweet{User: &User{ScreenName: "dghubble"}, Text: ".@audreyr use a DONTREADME file if you really want people to read it :P"}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweet)
}

func TestStatusService_ShowHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/show.json", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, map[string]string{"id": "589488862814076930"}, r)
	})
	client := NewClient(httpClient)
	client.Statuses.Show(589488862814076930, nil)
}

func TestStatusService_Lookup(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/lookup.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"id": "20,573893817000140800", "trim_user": "true"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"id": 20, "text": "just setting up my twttr"}, {"id": 573893817000140800, "text": "Don't get lost #PaxEast2015"}]`)
	})

	client := NewClient(httpClient)
	params := &StatusLookupParams{ID: []int64{20}, TrimUser: Bool(true)}
	tweets, _, err := client.Statuses.Lookup([]int64{573893817000140800}, params)
	expected := []Tweet{Tweet{ID: 20, Text: "just setting up my twttr"}, Tweet{ID: 573893817000140800, Text: "Don't get lost #PaxEast2015"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweets)
}

func TestStatusService_LookupHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()
	mux.HandleFunc("/1.1/statuses/lookup.json", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, map[string]string{"id": "20,573893817000140800"}, r)
	})
	client := NewClient(httpClient)
	client.Statuses.Lookup([]int64{20, 573893817000140800}, nil)
}

func TestStatusService_Update(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/update.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertQuery(t, map[string]string{}, r)
		assertPostForm(t, map[string]string{"status": "very informative tweet", "media_ids": "123456789,987654321", "lat": "37.826706", "long": "-122.42219"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 581980947630845953, "text": "very informative tweet"}`)
	})

	client := NewClient(httpClient)
	params := &StatusUpdateParams{MediaIds: []int64{123456789, 987654321}, Lat: Float(37.826706), Long: Float(-122.422190)}
	tweet, _, err := client.Statuses.Update("very informative tweet", params)
	expected := &Tweet{ID: 581980947630845953, Text: "very informative tweet"}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweet)
}

func TestStatusService_UpdateHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()
	mux.HandleFunc("/1.1/statuses/update.json", func(w http.ResponseWriter, r *http.Request) {
		assertPostForm(t, map[string]string{"status": "very informative tweet"}, r)
	})
	client := NewClient(httpClient)
	client.Statuses.Update("very informative tweet", nil)
}

func TestStatusService_APIError(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()
	mux.HandleFunc("/1.1/statuses/update.json", func(w http.ResponseWriter, r *http.Request) {
		assertPostForm(t, map[string]string{"status": "very informative tweet"}, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprintf(w, `{"errors": [{"message": "Status is a duplicate", "code": 187}]}`)
	})

	client := NewClient(httpClient)
	_, _, err := client.Statuses.Update("very informative tweet", nil)
	expected := APIError{
		Errors: []ErrorDetail{
			ErrorDetail{Message: "Status is a duplicate", Code: 187},
		},
	}
	if assert.Error(t, err) {
		assert.Equal(t, expected, err)
	}
}

func TestStatusService_HTTPError(t *testing.T) {
	httpClient, _, server := testServer()
	server.Close()
	client := NewClient(httpClient)
	_, _, err := client.Statuses.Update("very informative tweet", nil)
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("Statuses.Update error expected connection refused, got: \n %+v", err)
	}
}

func TestStatusService_Retweet(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/retweet/20.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertQuery(t, map[string]string{}, r)
		assertPostForm(t, map[string]string{"id": "20", "trim_user": "true"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 581980947630202020, "text": "RT @jack: just setting up my twttr", "retweeted_status": {"id": 20, "text": "just setting up my twttr"}}`)
	})

	client := NewClient(httpClient)
	params := &StatusRetweetParams{TrimUser: Bool(true)}
	tweet, _, err := client.Statuses.Retweet(20, params)
	expected := &Tweet{ID: 581980947630202020, Text: "RT @jack: just setting up my twttr", RetweetedStatus: &Tweet{ID: 20, Text: "just setting up my twttr"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweet)
}

func TestStatusService_RetweetHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/retweet/20.json", func(w http.ResponseWriter, r *http.Request) {
		assertPostForm(t, map[string]string{"id": "20"}, r)
	})

	client := NewClient(httpClient)
	client.Statuses.Retweet(20, nil)
}

func TestStatusService_Unretweet(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/unretweet/20.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertQuery(t, map[string]string{}, r)
		assertPostForm(t, map[string]string{"id": "20", "trim_user": "true"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 581980947630202020, "text":"RT @jack: just setting up my twttr", "retweeted_status": {"id": 20, "text": "just setting up my twttr"}}`)
	})

	client := NewClient(httpClient)
	params := &StatusUnretweetParams{TrimUser: Bool(true)}
	tweet, _, err := client.Statuses.Unretweet(20, params)
	expected := &Tweet{ID: 581980947630202020, Text: "RT @jack: just setting up my twttr", RetweetedStatus: &Tweet{ID: 20, Text: "just setting up my twttr"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweet)
}

func TestStatusService_Retweets(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/retweets/20.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"id": "20", "count": "2"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"text": "RT @jack: just setting up my twttr"}, {"text": "RT @jack: just setting up my twttr"}]`)
	})

	client := NewClient(httpClient)
	params := &StatusRetweetsParams{Count: 2}
	retweets, _, err := client.Statuses.Retweets(20, params)
	expected := []Tweet{Tweet{Text: "RT @jack: just setting up my twttr"}, Tweet{Text: "RT @jack: just setting up my twttr"}}
	assert.Nil(t, err)
	assert.Equal(t, expected, retweets)
}

func TestStatusService_RetweetsHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/retweets/20.json", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, map[string]string{"id": "20"}, r)
	})

	client := NewClient(httpClient)
	client.Statuses.Retweets(20, nil)
}

func TestStatusService_Destroy(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/destroy/40.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertQuery(t, map[string]string{}, r)
		assertPostForm(t, map[string]string{"id": "40", "trim_user": "true"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 40, "text": "wishing I had another sammich"}`)
	})

	client := NewClient(httpClient)
	params := &StatusDestroyParams{TrimUser: Bool(true)}
	tweet, _, err := client.Statuses.Destroy(40, params)
	// feed Biz Stone a sammich, he deletes sammich Tweet
	expected := &Tweet{ID: 40, Text: "wishing I had another sammich"}
	assert.Nil(t, err)
	assert.Equal(t, expected, tweet)
}

func TestStatusService_DestroyHandlesNilParams(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/destroy/40.json", func(w http.ResponseWriter, r *http.Request) {
		assertPostForm(t, map[string]string{"id": "40"}, r)
	})

	client := NewClient(httpClient)
	client.Statuses.Destroy(40, nil)
}

func TestStatusService_OEmbed(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/statuses/oembed.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"id": "691076766878691329", "maxwidth": "400", "hide_media": "true"}, r)
		w.Header().Set("Content-Type", "application/json")
		// abbreviated oEmbed response
		fmt.Fprintf(w, `{"url": "https://twitter.com/dghubble/statuses/691076766878691329", "width": 400, "html": "<blockquote></blockquote>"}`)
	})

	client := NewClient(httpClient)
	params := &StatusOEmbedParams{
		ID:        691076766878691329,
		MaxWidth:  400,
		HideMedia: Bool(true),
	}
	oembed, _, err := client.Statuses.OEmbed(params)
	expected := &OEmbedTweet{
		URL:   "https://twitter.com/dghubble/statuses/691076766878691329",
		Width: 400,
		HTML:  "<blockquote></blockquote>",
	}
	assert.Nil(t, err)
	assert.Equal(t, expected, oembed)
}
