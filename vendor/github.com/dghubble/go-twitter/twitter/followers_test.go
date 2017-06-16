package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFollowerService_Ids(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/followers/ids.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"user_id": "623265148", "count": "5", "cursor": "1516933260114270762"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ids":[178082406,3318241001,1318020818,191714329,376703838],"next_cursor":1516837838944119498,"next_cursor_str":"1516837838944119498","previous_cursor":-1516924983503961435,"previous_cursor_str":"-1516924983503961435"}`)
	})
	expected := &FollowerIDs{
		IDs:               []int64{178082406, 3318241001, 1318020818, 191714329, 376703838},
		NextCursor:        1516837838944119498,
		NextCursorStr:     "1516837838944119498",
		PreviousCursor:    -1516924983503961435,
		PreviousCursorStr: "-1516924983503961435",
	}

	client := NewClient(httpClient)
	params := &FollowerIDParams{
		UserID: 623265148,
		Count:  5,
		Cursor: 1516933260114270762,
	}
	followerIDs, _, err := client.Followers.IDs(params)
	assert.Nil(t, err)
	assert.Equal(t, expected, followerIDs)
}

func TestFollowerService_List(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/followers/list.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"screen_name": "dghubble", "count": "5", "cursor": "1516933260114270762", "skip_status": "true", "include_user_entities": "false"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"users": [{"id": 123}], "next_cursor":1516837838944119498,"next_cursor_str":"1516837838944119498","previous_cursor":-1516924983503961435,"previous_cursor_str":"-1516924983503961435"}`)
	})
	expected := &Followers{
		Users:             []User{User{ID: 123}},
		NextCursor:        1516837838944119498,
		NextCursorStr:     "1516837838944119498",
		PreviousCursor:    -1516924983503961435,
		PreviousCursorStr: "-1516924983503961435",
	}

	client := NewClient(httpClient)
	params := &FollowerListParams{
		ScreenName:          "dghubble",
		Count:               5,
		Cursor:              1516933260114270762,
		SkipStatus:          Bool(true),
		IncludeUserEntities: Bool(false),
	}
	followers, _, err := client.Followers.List(params)
	assert.Nil(t, err)
	assert.Equal(t, expected, followers)
}
