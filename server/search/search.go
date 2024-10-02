package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"server/models"
	"strings"

	"github.com/joho/godotenv"
)

func Search(q models.Query) (models.SearchResponse, error) {
	err := godotenv.Load(".env")
	if err != nil {
		return models.SearchResponse{}, fmt.Errorf(".env file not found: %s", err)
	}

	API_KEY := os.Getenv("CUSTOM_SEARCH_API_KEY")
	CSE_ID := os.Getenv("CUSTOM_SEARCH_CSE_ID")

	// better query
	searchQuery := fmt.Sprintf("\"%s vs %s\" %s site:reddit.com", q.Champion, q.Opponent, q.Role)
	searchURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?q=%s&key=%s&cx=%s&num=%d",
		url.QueryEscape(searchQuery),
		API_KEY, CSE_ID, 4) // Increased to 10 to have more results to filter

	fmt.Println(searchURL)

	resp, err := http.Get(searchURL)
	if err != nil {
		return models.SearchResponse{}, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	var searchResults models.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
		return models.SearchResponse{}, fmt.Errorf("failed to decode response: %v", err)
	}

	// filter irrelevant results
	filteredItems := filterSearchResults(searchResults.Items, q.Champion, q.Opponent)
	searchResults.Items = filteredItems

	return searchResults, nil
}

func filterSearchResults(items []models.SearchItem, champion, opponent string) []models.SearchItem {
	var filteredItems []models.SearchItem
	for _, item := range items {
		if isRelevantResult(item) {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems
}

func isRelevantResult(item models.SearchItem) bool {

	return !strings.Contains(item.Link, "NoStupidQuestions") || strings.Contains(item.Link, "RelayForReddit")

}
