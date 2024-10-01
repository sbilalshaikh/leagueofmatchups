package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
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

	key := q.Champion + "v" + q.Opponent + "@" + q.Role
	advice, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// continue if key isnt in cache
	} else if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Redis error: %s", err)})
		return
	} else {
		jsonResponse(w, http.StatusOK, map[string]string{"advice": advice})
		return
	}

	searchResults, err := search.Search(q)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Search failed: %s", err)})
		return
	}

	resultChan := make(chan struct {
		buffer bytes.Buffer
		errors []string
	})

	go func() {
		var wg sync.WaitGroup
		var mu sync.Mutex
		var buffer bytes.Buffer
		errors := make([]string, 0)

		for _, item := range searchResults.Items {
			wg.Add(1)
			go func(item models.SearchItem) {
				defer wg.Done()

				select {
				case <-ctx.Done():
					return // exit if context is finished
				default:
					scrapedContent, err := scrape.Scrape(item)
					if err != nil {
						mu.Lock()
						errors = append(errors, fmt.Sprintf("Scraping error for %s: %v", item.Link, err))
						mu.Unlock()
						return
					}

					summary, err := summarize.Summarize(scrapedContent, q.Champion, q.Opponent, q.Role)
					if err != nil {
						mu.Lock()
						errors = append(errors, fmt.Sprintf("Summarization error for %s: %v", item.Link, err))
						mu.Unlock()
						return
					}

					mu.Lock()
					buffer.WriteString(summary + "\n\n")
					mu.Unlock()
				}
			}(item)
		}

		wg.Wait()
		resultChan <- struct {
			buffer bytes.Buffer
			errors []string
		}{buffer, errors}
	}()

	select {
	case result := <-resultChan:
		// process completed
		if len(result.errors) > 0 {
			jsonResponse(w, http.StatusPartialContent, map[string]interface{}{
				"advice": result.buffer.String(),
				"errors": result.errors,
			})
			return
		}

		str := result.buffer.String()
		if strings.Contains(str, "INVALID-INPUT") {
			if err := rdb.Set(ctx, key, str, 2592000*time.Second).Err(); err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to set Redis key: %v", err)})
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"advice": "We aren't confident about the availability of advice on Reddit for this matchup :("})
			return
		}

		if err := rdb.Set(ctx, key, str, 2592000*time.Second).Err(); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to set Redis key: %v", err)})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"advice": str})

	case <-ctx.Done():
		// timeout occured
		jsonResponse(w, http.StatusRequestTimeout, map[string]string{"error": "Processing took too long and was terminated"})
		return
	}
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
