package twitter

// Entities represent metadata and context info parsed from Twitter components.
// https://dev.twitter.com/overview/api/entities
// TODO: symbols
type Entities struct {
	Hashtags     []HashtagEntity `json:"hashtags"`
	Media        []MediaEntity   `json:"media"`
	Urls         []URLEntity     `json:"urls"`
	UserMentions []MentionEntity `json:"user_mentions"`
}

// HashtagEntity represents a hashtag which has been parsed from text.
type HashtagEntity struct {
	Indices Indices `json:"indices"`
	Text    string  `json:"text"`
}

// URLEntity represents a URL which has been parsed from text.
type URLEntity struct {
	Indices     Indices `json:"indices"`
	DisplayURL  string  `json:"display_url"`
	ExpandedURL string  `json:"expanded_url"`
	URL         string  `json:"url"`
}

// MediaEntity represents media elements associated with a Tweet.
type MediaEntity struct {
	URLEntity
	ID                int64      `json:"id"`
	IDStr             string     `json:"id_str"`
	MediaURL          string     `json:"media_url"`
	MediaURLHttps     string     `json:"media_url_https"`
	SourceStatusID    int64      `json:"source_status_id"`
	SourceStatusIDStr string     `json:"source_status_id_str"`
	Type              string     `json:"type"`
	Sizes             MediaSizes `json:"sizes"`
	VideoInfo         VideoInfo  `json:"video_info"`
}

// MentionEntity represents Twitter user mentions parsed from text.
type MentionEntity struct {
	Indices    Indices `json:"indices"`
	ID         int64   `json:"id"`
	IDStr      string  `json:"id_str"`
	Name       string  `json:"name"`
	ScreenName string  `json:"screen_name"`
}

// UserEntities contain Entities parsed from User url and description fields.
// https://dev.twitter.com/overview/api/entities-in-twitter-objects#users
type UserEntities struct {
	URL         Entities `json:"url"`
	Description Entities `json:"description"`
}

// ExtendedEntity contains media information.
// https://dev.twitter.com/overview/api/entities-in-twitter-objects#extended_entities
type ExtendedEntity struct {
	Media []MediaEntity `json:"media"`
}

// Indices represent the start and end offsets within text.
type Indices [2]int

// Start returns the index at which an entity starts, inclusive.
func (i Indices) Start() int {
	return i[0]
}

// End returns the index at which an entity ends, exclusive.
func (i Indices) End() int {
	return i[1]
}

// MediaSizes contain the different size media that are available.
// https://dev.twitter.com/overview/api/entities#obj-sizes
type MediaSizes struct {
	Thumb  MediaSize `json:"thumb"`
	Large  MediaSize `json:"large"`
	Medium MediaSize `json:"medium"`
	Small  MediaSize `json:"small"`
}

// MediaSize describes the height, width, and resizing method used.
type MediaSize struct {
	Width  int    `json:"w"`
	Height int    `json:"h"`
	Resize string `json:"resize"`
}

// VideoInfo is available on video media objects.
type VideoInfo struct {
	AspectRatio    [2]int         `json:"aspect_ratio"`
	DurationMillis int            `json:"duration_millis"`
	Variants       []VideoVariant `json:"variants"`
}

// VideoVariant describes one of the available video formats.
type VideoVariant struct {
	ContentType string `json:"content_type"`
	Bitrate     int    `json:"bitrate"`
	URL         string `json:"url"`
}
