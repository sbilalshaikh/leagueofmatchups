package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"server/models"

	"github.com/joho/godotenv"
)

func Search(q models.Query) (models.SearchResponse, error) {

	err := godotenv.Load(".env")
	if err != nil {
		return models.SearchResponse{}, fmt.Errorf(".env file not found: %s", err)
	}

	API_KEY := os.Getenv("CUSTOM_SEARCH_API_KEY")
	CSE_ID := os.Getenv("CUSTOM_SEARCH_CSE_ID")

	searchQuery := fmt.Sprintf("%s v %s %s reddit", q.Champion, q.Opponent, q.Role)
	searchURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?q=%s&key=%s&cx=%s&num=%d",
		url.QueryEscape(searchQuery),
		API_KEY, CSE_ID, 4)

	resp, err := http.Get(searchURL)
	if err != nil {
		return models.SearchResponse{}, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	var searchResults models.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
		return models.SearchResponse{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return searchResults, nil

}
