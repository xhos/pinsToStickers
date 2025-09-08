package pinterest

import "time"

type Pin struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	LargeURL    string    `json:"large_url"`
	OriginalURL string    `json:"original_url"`
	BoardID     string    `json:"board_id"`
	BoardName   string    `json:"board_name"`
	Username    string    `json:"username"`
	CreatedAt   time.Time `json:"created_at"`
	Position    int       `json:"position"`
}

type Board struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	PinCount    int    `json:"pin_count"`
}

type GalleryDLMetadata struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Extension   string `json:"extension"`
	Filename    string `json:"filename"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Board       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"board"`
	User struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"user"`
	Date      string `json:"date"`
	Repin     bool   `json:"repin"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	MediaType string `json:"media_type"`
}

type DownloadResult struct {
	Pins        []Pin
	Board       Board
	SuccessCount int
	FailedCount  int
	Errors       []error
}