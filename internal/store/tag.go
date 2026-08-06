package store

// Tag is the lightweight tag representation shared by ComicSeries metadata.
type Tag struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
