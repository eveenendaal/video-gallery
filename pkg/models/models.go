package models

// Category represents a category of galleries
type Category struct {
	Name      string    `json:"name"`
	Stub      string    `json:"stub"`
	Galleries []Gallery `json:"galleries"`
}

// Gallery represents a collection of videos
type Gallery struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Stub     string  `json:"-"`
	Videos   []Video `json:"videos"`
}

// Video represents a video file with an optional thumbnail
type Video struct {
	Name      string  `json:"name"`
	Category  string  `json:"-"`
	Gallery   string  `json:"-"`
	Url       string  `json:"url"`
	Thumbnail *string `json:"thumbnail,omitempty"`
}

// Index represents the main index page data
type Index struct {
	Categories []Category
}
