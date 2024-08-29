package summarize

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type Comment struct {
	Timestamp int64
	Content   string
	Permalink string
	Score     int
	Replies   []Comment // For nested comments
}

type Post struct {
	Timestamp int64
	Content   string
	Permalink string
	Title     string
	Score     int
	Comments  []Comment
}

func formatPostContent(post Post) (string, error) {
	var sb strings.Builder

	entry, err := formatEntry(post.Timestamp, post.Title, post.Permalink, post.Score, post.Content, 0)
	if err != nil {
		return "", fmt.Errorf("error formatting post: %w", err)
	}
	sb.WriteString(entry)

	topComments := getTopComments(post.Comments, 5)

	for _, comment := range topComments {
		entry, err := formatEntry(comment.Timestamp, "", comment.Permalink, comment.Score, comment.Content, 1)
		if err != nil {
			return "", fmt.Errorf("error formatting comment: %w", err)
		}
		sb.WriteString(entry)

		topReplies := getTopComments(comment.Replies, 2)

		for _, reply := range topReplies {
			entry, err := formatEntry(reply.Timestamp, "", reply.Permalink, reply.Score, reply.Content, 2)
			if err != nil {
				return "", fmt.Errorf("error formatting reply: %w", err)
			}
			sb.WriteString(entry)
		}
	}

	return sb.String(), nil
}

func formatEntry(timestamp int64, title, permalink string, score int, content string, indentLevel int) (string, error) {
	indent := strings.Repeat("\t", indentLevel)
	dateStr := time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")

	var titleStr string
	if title != "" {
		titleStr = fmt.Sprintf("[%s] ", title)
	}

	if permalink == "" {
		return "", fmt.Errorf("empty permalink")
	}

	return fmt.Sprintf("%s[%s] %s[%s] [%d] {%s}\n", indent, dateStr, titleStr, permalink, score, content), nil
}

func getTopComments(comments []Comment, n int) []Comment {
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].Score > comments[j].Score
	})

	n = min(len(comments), n)
	return comments[:n]
}

func Summarize(data []byte, championA string, championB string, role string) (string, error) {
	var post Post
	err := json.Unmarshal(data, &post)
	if err != nil {
		return "", fmt.Errorf("couldn't convert json to post: %s", err)
	}
	formattedPost, err := formatPostContent(post)
	if err != nil {
		return "", fmt.Errorf("couldn't format reddit post correctly: %s", err)
	}

	systemPrompt := fmt.Sprintf(`
        You are an expert League of Legends analyst. Given the following comments and subcomments about a %s vs %s matchup in the %s role, please:
        1. Consider both main comments and subcomments in your analysis
        2. Filter out non-productive or irrelevant comments
        3. Give more weight to recent comments
        4. Give more weight to comments with higher score
        5. Generate a summary with 3-5 bullet points
        6. Cite all relevant sources (links) for each point in the summary
        7. keep a formal mood and third person

        Data format:
        [timestamp] [post title] [postlink] [score] [post content]
            [timestamp] [comment link] [score] [comment content]
                [timestamp] [subcomment link] [score] [subcomment content]

        Your response should be formatted as follows:
        • Summary point 1 [Sources: [link1, link2, ...]]
        • Summary point 2 [Sources: [link3, link4, ...]]
        • Summary point 3 [Sources: [link5, link6, ...]]
        • (Additional points if necessary)

        Important:
        - Provide at least 4 summary points, but no more than 7
        - Include multiple sources for each point when available
        - Concatenate "www.reddit.com" to the beginning of each link
        - If the matchup is reversed in the content, adjust your advice accordingly

        Respond with ONLY THE SUMMARY, formatted as specified above.
    `, championA, championB, role)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	reqbody, err := json.Marshal(map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        2200,
		"system":            systemPrompt,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "text",
						"text": "Analyze the following data:\n\n" + formattedPost,
					},
				},
			},
		},
		"temperature": 0,
		"top_p":       0.5,
	})
	if err != nil {
		return "", fmt.Errorf("error creating request body: %v", err)
	}

	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	resp, err := bedrockClient.InvokeModel(context.TODO(), &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("anthropic.claude-3-sonnet-20240229-v1:0"),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        reqbody,
	})

	if err != nil {
		return "", fmt.Errorf("couldn't hit bedrock properly: %s", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body, &result)
	if err != nil {
		return "", fmt.Errorf("couldn't unmarshal the result: %s", err)
	}

	completion, ok := result["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !ok {
		return "", fmt.Errorf("completion not found in the response or not a string")
	}

	return completion, nil
}
