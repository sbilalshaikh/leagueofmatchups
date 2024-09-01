package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"server/models"
	"server/scrape"
	"server/search"
	"server/summarize"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var (
	ctx context.Context
	rdb *redis.Client
)

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
	ctx = context.Background()

	if err := godotenv.Load(".env"); err != nil {
		// handle it in matchup hanlder
	}

	if err := initRedis(); err != nil {
		// handle it in matchup handler not here
	}
}

func MatchupHandler(c *gin.Context) {
	if rdb == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis client not initialized"})
		return
	}

	q := models.Query{

		Champion: c.Query("champ"),
		Opponent: c.Query("opp"),
		Role:     c.Query("role"),
	}

	key := q.Champion + "v" + q.Opponent + "@" + q.Role
	advice, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Redis error: %s", err)})
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"advice": advice})
		return
	}

	searchResults, err := search.Search(q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Search failed: %s", err)})
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var buffer bytes.Buffer
	errors := make([]string, 0)

	for _, item := range searchResults.Items {
		wg.Add(1)
		go func(item models.SearchItem) {
			defer wg.Done()

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
		}(item)
	}
	wg.Wait()

	if len(errors) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{
			"advice": buffer.String(),
			"errors": errors,
		})
		return
	}

	str := buffer.String()
	if strings.Contains(str, "INVALID-INPUT") {
		if err := rdb.Set(ctx, key, str, 2592000*time.Second).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to set Redis key: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"advice": "We aren't confident about the availability of advice on Reddit for this matchup :("})
		return
	}

	if err := rdb.Set(ctx, key, str, 2592000*time.Second).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to set Redis key: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"advice": buffer.String()})
}

func main() {
	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"https://leagueofmatchups.ai"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	config.AllowCredentials = true
	config.ExposeHeaders = []string{"Content-Length"}
	config.MaxAge = 12 * time.Hour

	r.Use(cors.New(config))

	routes := r.Group("/api")
	{
		routes.GET("/matchup", MatchupHandler)
	}

	if err := r.Run(":8080"); err != nil {
		// print to stderr if we cant start
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
