package models

type Query struct {
	Champion string `json:"champ"`
	Opponent string `json:"opp"`
	Role     string `json:"role"`
}

type SearchResponse struct {
	Items []SearchItem `json:"items"`
}

type SearchItem struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	Snippet      string `json:"snippet"`
	FormattedURL string `json:"formattedUrl"`
}

type Comment struct {
	Timestamp string    `json:"timestamp"`
	Content   string    `json:"content"`
	Link      string    `json:"link"`
	Replies   []Comment `json:"replies"`
}

type Post struct {
	Timestamp string    `json:"timestamp"`
	Content   string    `json:"content"`
	Link      string    `json:"link"`
	Comments  []Comment `json:"comments"`
}
