package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testDM = DirectMessage{
		ID:        240136858829479936,
		Recipient: &User{ScreenName: "theSeanCook"},
		Sender:    &User{ScreenName: "s0c1alm3dia"},
		Text:      "hello world",
	}
	testDMIDStr = "240136858829479936"
	testDMJSON  = `{"id": 240136858829479936,"recipient": {"screen_name": "theSeanCook"},"sender": {"screen_name": "s0c1alm3dia"},"text": "hello world"}`
)

func TestDirectMessageService_Show(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/direct_messages/show.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"id": testDMIDStr}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, testDMJSON)
	})

	client := NewClient(httpClient)
	dms, _, err := client.DirectMessages.Show(testDM.ID)
	assert.Nil(t, err)
	assert.Equal(t, &testDM, dms)
}

func TestDirectMessageService_Get(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/direct_messages.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"since_id": "589147592367431680", "count": "1"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[`+testDMJSON+`]`)
	})

	client := NewClient(httpClient)
	params := &DirectMessageGetParams{SinceID: 589147592367431680, Count: 1}
	dms, _, err := client.DirectMessages.Get(params)
	expected := []DirectMessage{testDM}
	assert.Nil(t, err)
	assert.Equal(t, expected, dms)
}

func TestDirectMessageService_Sent(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/direct_messages/sent.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"since_id": "589147592367431680", "count": "1"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[`+testDMJSON+`]`)
	})

	client := NewClient(httpClient)
	params := &DirectMessageSentParams{SinceID: 589147592367431680, Count: 1}
	dms, _, err := client.DirectMessages.Sent(params)
	expected := []DirectMessage{testDM}
	assert.Nil(t, err)
	assert.Equal(t, expected, dms)
}

func TestDirectMessageService_New(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/direct_messages/new.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertPostForm(t, map[string]string{"screen_name": "theseancook", "text": "hello world"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, testDMJSON)
	})

	client := NewClient(httpClient)
	params := &DirectMessageNewParams{ScreenName: "theseancook", Text: "hello world"}
	dm, _, err := client.DirectMessages.New(params)
	assert.Nil(t, err)
	assert.Equal(t, &testDM, dm)
}

func TestDirectMessageService_Destroy(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/direct_messages/destroy.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "POST", r)
		assertPostForm(t, map[string]string{"id": testDMIDStr}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, testDMJSON)
	})

	client := NewClient(httpClient)
	dm, _, err := client.DirectMessages.Destroy(testDM.ID, nil)
	assert.Nil(t, err)
	assert.Equal(t, &testDM, dm)
}
