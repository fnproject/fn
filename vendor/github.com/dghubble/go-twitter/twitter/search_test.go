package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchService_Tweets(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/search/tweets.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"q": "happy birthday", "result_type": "popular", "count": "1"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"statuses":[{"id":781760642139250689}],"search_metadata":{"completed_in":0.043,"max_id":781760642139250689,"max_id_str":"781760642139250689","next_results":"?max_id=781760640104828927&q=happy+birthday&count=1&include_entities=1","query":"happy birthday","refresh_url":"?since_id=781760642139250689&q=happy+birthday&include_entities=1","count":1,"since_id":0,"since_id_str":"0"}}`)
	})

	client := NewClient(httpClient)
	search, _, err := client.Search.Tweets(&SearchTweetParams{
		Query:      "happy birthday",
		Count:      1,
		ResultType: "popular",
	})
	expected := &Search{
		Statuses: []Tweet{
			Tweet{ID: 781760642139250689},
		},
		Metadata: &SearchMetadata{
			Count:       1,
			SinceID:     0,
			SinceIDStr:  "0",
			MaxID:       781760642139250689,
			MaxIDStr:    "781760642139250689",
			RefreshURL:  "?since_id=781760642139250689&q=happy+birthday&include_entities=1",
			NextResults: "?max_id=781760640104828927&q=happy+birthday&count=1&include_entities=1",
			CompletedIn: 0.043,
			Query:       "happy birthday",
		},
	}
	assert.Nil(t, err)
	assert.Equal(t, expected, search)
}
