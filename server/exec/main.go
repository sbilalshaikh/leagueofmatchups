package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"server/models"
	"server/scrape"
	"server/search"
	"server/summarize"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var rdb *redis.Client

func initRedis() error {
	redisEndpt := os.Getenv("REDIS_ENDPOINT")
	if redisEndpt == "" {
		return fmt.Errorf("REDIS_ENDPOINT environment variable is not set")
	}

	rdb = redis.NewClient(&redis.Options{
		Addr: redisEndpt,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return nil
}

func init() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Error loading .env file:", err)
	}

	if err := initRedis(); err != nil {
		log.Println("Error initializing Redis:", err)
	}
}

func jsonResponse(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func MatchupHandler(w http.ResponseWriter, r *http.Request) {
	// 3 minute timeout context
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	if rdb == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Redis client not initialized"})
		return
	}

	q := models.Query{
		Champion: r.URL.Query().Get("champ"),
		Opponent: r.URL.Query().Get("opp"),
		Role:     r.URL.Query().Get("role"),
	}

	// Validate input
	if q.Champion == "" || q.Opponent == "" || q.Role == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing required parameters"})
		return
	}

	key := q.Champion + "v" + q.Opponent + "@" + q.Role
	advice, err := rdb.Get(ctx, key).Result()
	if err == nil {
		// If key exists in cache, return it immediately
		jsonResponse(w, http.StatusOK, map[string]string{"advice": advice})
		return
	} else if err != redis.Nil {
		// If there's an error other than key not existing, return error
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Redis error: %s", err)})
		return
	}

	// If we're here, the key wasn't in the cache, so we need to generate advice

	searchResults, err := search.Search(q)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Search failed: %s", err)})
		return
	}

	if len(searchResults.Items) == 0 {
		advice := "We aren't confident about the availability of advice on Reddit for this matchup :("
		if err := rdb.Set(ctx, key, advice, 2592000*time.Second).Err(); err != nil {
			log.Printf("Failed to set Redis key: %v", err)
		}
		jsonResponse(w, http.StatusOK, map[string]string{"advice": advice})
		return
	}

	resultChan := make(chan string)
	errorChan := make(chan error)

	for _, item := range searchResults.Items {
		go func(item models.SearchItem) {
			scrapedContent, err := scrape.Scrape(item)
			if err != nil {
				errorChan <- fmt.Errorf("scraping error for %s: %v", item.Link, err)
				return
			}

			summary, err := summarize.Summarize(scrapedContent, q.Champion, q.Opponent, q.Role)
			if err != nil {
				errorChan <- fmt.Errorf("summarization error for %s: %v", item.Link, err)
				return
			}

			if strings.Contains(summary, "INVALID_INPUT") {
				errorChan <- fmt.Errorf("invalid input for %s", item.Link)
				return
			}

			resultChan <- summary
		}(item)
	}

	var finalAdvice strings.Builder
	errorCount := 0

	for i := 0; i < len(searchResults.Items); i++ {
		select {
		case summary := <-resultChan:
			finalAdvice.WriteString(summary)
			finalAdvice.WriteString("\n\n")
		case err := <-errorChan:
			log.Printf("Error: %v", err)
			errorCount++
		case <-ctx.Done():
			jsonResponse(w, http.StatusRequestTimeout, map[string]string{"error": "Processing took too long and was terminated"})
			return
		}
	}

	if finalAdvice.Len() == 0 || errorCount == len(searchResults.Items) {
		advice := "We aren't confident about the availability of advice on Reddit for this matchup :("
		if err := rdb.Set(ctx, key, advice, 2592000*time.Second).Err(); err != nil {
			log.Printf("Failed to set Redis key: %v", err)
		}
		jsonResponse(w, http.StatusOK, map[string]string{"advice": advice})
		return
	}

	advice = finalAdvice.String()
	if err := rdb.Set(ctx, key, advice, 2592000*time.Second).Err(); err != nil {
		log.Printf("Failed to set Redis key: %v", err)
	}
	jsonResponse(w, http.StatusOK, map[string]string{"advice": advice})
}

func main() {
	http.HandleFunc("/api/matchup", MatchupHandler)

	srv := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "https://leagueofmatchups.ai")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			http.DefaultServeMux.ServeHTTP(w, r)
		}),
	}

	// init  server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v\n", err)
		}
	}()

	// wait for singal to shutdown server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
}
