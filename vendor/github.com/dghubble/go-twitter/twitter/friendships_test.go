package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFriendshipService_Create(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/friendships/create.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertPostForm(t, map[string]string{"user_id": "12345"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 12345, "name": "Doug Williams"}`)
	})

	client := NewClient(httpClient)
	params := &FriendshipCreateParams{UserID: 12345}
	user, _, err := client.Friendships.Create(params)
	assert.Nil(t, err)
	expected := &User{ID: 12345, Name: "Doug Williams"}
	assert.Equal(t, expected, user)
}

func TestFriendshipService_Show(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/friendships/show.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"source_screen_name": "foo", "target_screen_name": "bar"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{ "relationship": { "source": { "can_dm": false, "muting": true, "id_str": "8649302", "id": 8649302, "screen_name": "foo"}, "target": { "id_str": "12148", "id": 12148, "screen_name": "bar", "following": true, "followed_by": false } } }`)
	})

	client := NewClient(httpClient)
	params := &FriendshipShowParams{SourceScreenName: "foo", TargetScreenName: "bar"}
	relationship, _, err := client.Friendships.Show(params)
	assert.Nil(t, err)
	expected := &Relationship{
		Source: RelationshipSource{ID: 8649302, ScreenName: "foo", IDStr: "8649302", CanDM: false, Muting: true, WantRetweets: false},
		Target: RelationshipTarget{ID: 12148, ScreenName: "bar", IDStr: "12148", Following: true, FollowedBy: false},
	}
	assert.Equal(t, expected, relationship)
}

func TestFriendshipService_Destroy(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/friendships/destroy.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertPostForm(t, map[string]string{"user_id": "12345"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": 12345, "name": "Doug Williams"}`)
	})

	client := NewClient(httpClient)
	params := &FriendshipDestroyParams{UserID: 12345}
	user, _, err := client.Friendships.Destroy(params)
	assert.Nil(t, err)
	expected := &User{ID: 12345, Name: "Doug Williams"}
	assert.Equal(t, expected, user)
}

func TestFriendshipService_Outgoing(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/friendships/outgoing.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"cursor": "1516933260114270762"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ids":[178082406,3318241001,1318020818,191714329,376703838],"next_cursor":1516837838944119498,"next_cursor_str":"1516837838944119498","previous_cursor":-1516924983503961435,"previous_cursor_str":"-1516924983503961435"}`)
	})
	expected := &FriendIDs{
		IDs:               []int64{178082406, 3318241001, 1318020818, 191714329, 376703838},
		NextCursor:        1516837838944119498,
		NextCursorStr:     "1516837838944119498",
		PreviousCursor:    -1516924983503961435,
		PreviousCursorStr: "-1516924983503961435",
	}

	client := NewClient(httpClient)
	params := &FriendshipPendingParams{
		Cursor: 1516933260114270762,
	}
	friendIDs, _, err := client.Friendships.Outgoing(params)
	assert.Nil(t, err)
	assert.Equal(t, expected, friendIDs)
}

func TestFriendshipService_Incoming(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/friendships/incoming.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"cursor": "1516933260114270762"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ids":[178082406,3318241001,1318020818,191714329,376703838],"next_cursor":1516837838944119498,"next_cursor_str":"1516837838944119498","previous_cursor":-1516924983503961435,"previous_cursor_str":"-1516924983503961435"}`)
	})
	expected := &FriendIDs{
		IDs:               []int64{178082406, 3318241001, 1318020818, 191714329, 376703838},
		NextCursor:        1516837838944119498,
		NextCursorStr:     "1516837838944119498",
		PreviousCursor:    -1516924983503961435,
		PreviousCursorStr: "-1516924983503961435",
	}

	client := NewClient(httpClient)
	params := &FriendshipPendingParams{
		Cursor: 1516933260114270762,
	}
	friendIDs, _, err := client.Friendships.Incoming(params)
	assert.Nil(t, err)
	assert.Equal(t, expected, friendIDs)
}
