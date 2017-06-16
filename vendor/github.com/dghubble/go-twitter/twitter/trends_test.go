package twitter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrendsService_Available(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/trends/available.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"country": "Sweden","countryCode": "SE","name": "Sweden","parentid": 1,"placeType": {"code": 12,"name": "Country"},"url": "http://where.yahooapis.com/v1/place/23424954","woeid": 23424954}]`)
	})
	expected := []Location{
		Location{
			Country:     "Sweden",
			CountryCode: "SE",
			Name:        "Sweden",
			ParentID:    1,
			PlaceType:   PlaceType{Code: 12, Name: "Country"},
			URL:         "http://where.yahooapis.com/v1/place/23424954",
			WOEID:       23424954,
		},
	}

	client := NewClient(httpClient)
	locations, _, err := client.Trends.Available()
	assert.Nil(t, err)
	assert.Equal(t, expected, locations)
}

func TestTrendsService_Place(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/trends/place.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"id": "123456"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"trends":[{"name":"#gotwitter"}], "as_of": "2017-02-08T16:18:18Z", "created_at": "2017-02-08T16:10:33Z","locations":[{"name": "Worldwide","woeid": 1}]}]`)
	})
	expected := []TrendsList{TrendsList{
		Trends:    []Trend{Trend{Name: "#gotwitter"}},
		AsOf:      "2017-02-08T16:18:18Z",
		CreatedAt: "2017-02-08T16:10:33Z",
		Locations: []TrendsLocation{TrendsLocation{Name: "Worldwide", WOEID: 1}},
	}}

	client := NewClient(httpClient)
	places, _, err := client.Trends.Place(123456, &TrendsPlaceParams{})
	assert.Nil(t, err)
	assert.Equal(t, expected, places)
}

func TestTrendsService_Closest(t *testing.T) {
	httpClient, mux, server := testServer()
	defer server.Close()

	mux.HandleFunc("/1.1/trends/closest.json", func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, "GET", r)
		assertQuery(t, map[string]string{"lat": "37.781157", "long": "-122.400612831116"}, r)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"country": "Sweden","countryCode": "SE","name": "Sweden","parentid": 1,"placeType": {"code": 12,"name": "Country"},"url": "http://where.yahooapis.com/v1/place/23424954","woeid": 23424954}]`)
	})
	expected := []Location{
		Location{
			Country:     "Sweden",
			CountryCode: "SE",
			Name:        "Sweden",
			ParentID:    1,
			PlaceType:   PlaceType{Code: 12, Name: "Country"},
			URL:         "http://where.yahooapis.com/v1/place/23424954",
			WOEID:       23424954,
		},
	}

	client := NewClient(httpClient)
	locations, _, err := client.Trends.Closest(&ClosestParams{Lat: 37.781157, Long: -122.400612831116})
	assert.Nil(t, err)
	assert.Equal(t, expected, locations)
}
