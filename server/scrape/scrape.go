package scrape

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"server/models"
	"strings"

	"github.com/joho/godotenv"
)

type Comment struct {
	Timestamp int64
	Content   string
	Permalink string
	Score     int
	Replies   []Comment
}

type Post struct {
	Timestamp int64
	Content   string
	Permalink string
	Title     string
	Score     int
	Comments  []Comment
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func getPostInfo(searchItem models.SearchItem) (string, string, error) {

	splitUrl := strings.Split(searchItem.Link[8:], "/")

	if splitUrl[0] == "www.reddit.com" && splitUrl[1] == "r" && splitUrl[3] == "comments" {
		return splitUrl[4], splitUrl[2], nil
	}

	return "", "", fmt.Errorf("url: %s was not formatted properly", searchItem.Link[0:8])

}

// returns the http client too to preserve the cache because that makes it faster I think
func getToken() (TokenResponse, *http.Client, error) {

	// environment variable stuff
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("error loading .env: %s", err)
		return TokenResponse{}, &http.Client{}, err
	}

	redditClientID := os.Getenv("REDDIT_CLIENT_ID")
	redditClientSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	redditUsername := os.Getenv("REDDIT_CLIENT_USERNAME")
	redditPassword := os.Getenv("REDDIT_CLIENT_PASSWORD")
	redditAppName := os.Getenv("REDDIT_APP_NAME")

	// prep http client & oauth2 stuff
	httpClient := &http.Client{}
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", redditUsername)
	data.Set("password", redditPassword)

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		log.Printf("error creating request: %s", err)
		return TokenResponse{}, &http.Client{}, err
	}

	req.SetBasicAuth(redditClientID, redditClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", fmt.Sprintf("%s by /u/%s", redditAppName, redditUsername))

	// send & deal with request
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("error making request: %s", err)
		return TokenResponse{}, &http.Client{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("error response: %s", resp.Status)
		return TokenResponse{}, &http.Client{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("couldnt read body: %s", err)
		return TokenResponse{}, &http.Client{}, err

	}

	// get the token
	var token TokenResponse
	err = json.Unmarshal(body, &token)
	if err != nil {
		log.Printf("error decoding response: %s", err)
		return TokenResponse{}, &http.Client{}, err
	}

	return token, httpClient, nil

}

func parseJson(data []interface{}) (*Post, error) {

	if len(data) < 2 {
		return nil, fmt.Errorf("insufficient data")
	}

	postData, ok := data[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid post data format")
	}

	commentsData, ok := data[1].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid comments data format")
	}

	post, err := parsePost(postData)
	if err != nil {
		return nil, err
	}

	comments, err := parseComments(commentsData)
	if err != nil {
		return nil, err
	}

	post.Comments = comments

	return post, nil

}

func parsePost(postData map[string]interface{}) (*Post, error) {
	children, ok := postData["data"].(map[string]interface{})["children"].([]interface{})
	if !ok || len(children) == 0 {
		return nil, fmt.Errorf("invalid post children data")
	}

	postMap, ok := children[0].(map[string]interface{})["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid post map data")
	}

	post := &Post{
		Timestamp: int64(postMap["created_utc"].(float64)),
		Content:   postMap["selftext"].(string),
		Permalink: postMap["permalink"].(string),
		Title:     postMap["title"].(string),
		Score:     int(postMap["score"].(float64)),
	}

	return post, nil
}

func parseComments(commentsData map[string]interface{}) ([]Comment, error) {
	data, ok := commentsData["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid comments data structure")
	}

	children, ok := data["children"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid comments children data")
	}

	var comments []Comment
	for _, child := range children {
		childMap, ok := child.(map[string]interface{})
		if !ok {
			continue // Skip invalid child
		}

		commentData, ok := childMap["data"].(map[string]interface{})
		if !ok {
			continue // Skip invalid comment data
		}

		comment, err := parseComment(commentData)
		if err != nil {
			// Log the error but continue processing other comments
			log.Printf("Error parsing comment: %v", err)
			continue
		}

		replies, ok := commentData["replies"].(map[string]interface{})
		if ok {
			subComments, err := parseComments(replies)
			if err == nil {
				comment.Replies = subComments
			} else {
				log.Printf("Error parsing replies: %v", err)
			}
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

func parseComment(commentData map[string]interface{}) (Comment, error) {
	var comment Comment
	var err error

	comment.Timestamp, err = getInt64(commentData, "created_utc")
	if err != nil {
		return Comment{}, err
	}

	comment.Content, err = getString(commentData, "body")
	if err != nil {
		return Comment{}, err
	}

	comment.Permalink, err = getString(commentData, "permalink")
	if err != nil {
		return Comment{}, err
	}

	comment.Score, err = getInt(commentData, "score")
	if err != nil {
		return Comment{}, err
	}

	return comment, nil
}

func getInt64(m map[string]interface{}, key string) (int64, error) {
	v, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("key %s not found", key)
	}
	switch i := v.(type) {
	case float64:
		return int64(i), nil
	case int64:
		return i, nil
	default:
		return 0, fmt.Errorf("unexpected type for key %s", key)
	}
}

func getString(m map[string]interface{}, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("key %s not found", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("value for key %s is not a string", key)
	}
	return s, nil
}

func getInt(m map[string]interface{}, key string) (int, error) {
	v, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("key %s not found", key)
	}
	switch i := v.(type) {
	case float64:
		return int(i), nil
	case int:
		return i, nil
	default:
		return 0, fmt.Errorf("unexpected type for key %s", key)
	}
}

func Scrape(item models.SearchItem) ([]byte, error) {

	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("error loading .env: %s", err)
		return []byte{}, fmt.Errorf("%s", err)
	}

	redditAppName := os.Getenv("REDDIT_APP_NAME")
	redditUsername := os.Getenv("REDDIT_CLIENT_USERNAME")

	postID, subreddit, err := getPostInfo(item)
	if err != nil {
		return []byte{}, fmt.Errorf("%s", err)
	}

	token, httpClient, err := getToken()
	if err != nil {
		return []byte{}, fmt.Errorf("error getting token: %s", err)
	}

	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/comments/%s", subreddit, postID)
	fmt.Println(url)

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return []byte{}, fmt.Errorf("couldnt make request: %s", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Set("User-Agent", fmt.Sprintf("%s by /u/%s", redditAppName, redditUsername))

	response, err := httpClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("%s", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("unexpected status code when reading post: %d", response.StatusCode)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("error reading response body: %w", err)
	}

	var result []interface{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return []byte{}, fmt.Errorf("couldnt unmarshall json: %s", err)
	}

	post, err := parseJson(result)
	if err != nil {
		return []byte{}, fmt.Errorf("couldnt parse json: %s", err)
	}

	postJson, err := json.MarshalIndent(post, "", "  ")
	if err != nil {
		return []byte{}, fmt.Errorf("error marshalling to JSON: %s", err)

	}

	return postJson, nil

}
