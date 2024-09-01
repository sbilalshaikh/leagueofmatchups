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

func performQualityControl(summary string, championA string, championB string) (string, error) {
	qualityControlPrompt := fmt.Sprintf(`
        You are an expert League of Legends analyst. The following summary needs to be checked for relevance and phrasing:

        1. Remove any points that are irrelevant to a matchup between %s (champion) and %s (opponent).
        2. If a point discusses the inverse matchup (%s vs %s), adjust the phrasing to reflect the correct perspective.
		3. Do not omit the sources
        4. Ignore all points that say "INVALID-INPUT".
        5. Use only the provided summary as the knowledge source; do not introduce any other information.
        6. Remove any points that do not discuss the direct relationship between %s (champion) and %s (opponent).
        7. Do not discuss anything about Riot Games' decisions.
        9. Omit any points that require discussing balance or Riot Games; focus only on the matchup.
        9. If you cannot revise a summary, write "INVALID-INPUT".
		10. Omit all meta commentary, ie only give the revised summary without offering any comments about it
		11. If the summary need not any revisions, output it as is 
        12. If the subreddit name is of the form "r/%smains", omit the point entirely
		13. Make sure there is a new line after each point
		14. Make sure there are no bullet points
		15. <BOLD> MAKE SURE ONLY THE MATCHUP BETWEEN  %s (champion) and %s (opponent) IS DISCUSSED </BOLD>
		17. <BOLD> Do not omit the sources </BOLD>
		18. <BOLD> Do not omit the sources </BOLD>
		19. <BOLD> Do not omit the sources </BOLD>
		20. <BOLD> Do not omit the sources </BOLD>
		21. <BOLD> Omit entries that contain champions that arent %s (champion) and %s (opponent

        Summary:
        %s

        Respond with ONLY the revised summary, formatted in bullet points as specified before.
    `, championA, championB, championB, championA, championA, championB, championA, championA, championB, championA, championB, summary)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		return "", fmt.Errorf("unable to load SDK config, %v", err)
	}

	reqbody, err := json.Marshal(map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        2200,
		"system":            qualityControlPrompt,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "text",
						"text": summary,
					},
				},
			},
		},
		"temperature": 0,
		"top_p":       0.5,
	})
	if err != nil {
		return "", fmt.Errorf("error creating quality control request body: %v", err)
	}

	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	resp, err := bedrockClient.InvokeModel(context.TODO(), &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("anthropic.claude-3-5-sonnet-20240620-v1:0"),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        reqbody,
	})

	if err != nil {
		return "", fmt.Errorf("couldn't perform quality control properly: %s", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body, &result)
	if err != nil {
		return "", fmt.Errorf("couldn't unmarshal the quality control result: %s", err)
	}

	qualityControlledCompletion, ok := result["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !ok {
		return "", fmt.Errorf("quality-controlled completion not found in the response or not a string")
	}

	return qualityControlledCompletion, nil
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
        5. Generate a summary with 1-2 bullet points
        6. Cite all relevant sources (links) for each point in the summary
        7. keep a formal mood and third person


		The data will be given as follows:
        <input-data-format>
        [timestamp] [post title] [postlink] [score] [post content]
            [timestamp] [comment link] [score] [comment content]
                [timestamp] [subcomment link] [score] [subcomment content]
        <input-data-format/>


        Your response should be formatted as follows :
        • {content} [Sources: [link1, link2, ...]]
        • {content}  [Sources: [link3, link4, ...]]
        • {content}  [Sources: [link5, link6, ...]]
        • (Additional points if necessary)



        Important:
        - Provide as many summary points as possible, but no more than 3
        - Include multiple sources for each point when available
        - Concatenate "www.reddit.com" to the beginning of each link
        - If the matchup is reversed in the content, adjust your advice accordingly
		- If the input text contains <txt>loreoflegends<txt/> or <txt>leagueofmemes</txt> output "INVALID-INPUT"
		- If the text is completely irrelevant to matchup between %s and %s output "INVALID-INPUT"
		- Ommit "summary points" in the output
		- <very-important> The only league of legends characters that should be mentioned are <champion>%s</champion> and <opponent>%s</opponent> </very-important>
		- <very-important> There should be no XML tags or special unicode characters (that have to be specified with /u) in the output </very-important>

        Respond with ONLY THE SUMMARY OR "INVALID_INPUT", formatted as specified above.
    `, championA, championB, role, championA, championB, championA, championB)

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
						"text": formattedPost,
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
		ModelId:     aws.String("anthropic.claude-3-5-sonnet-20240620-v1:0"),
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

	qualityControlledCompletion, err := performQualityControl(completion, championA, championB)
	if err != nil {
		return "", fmt.Errorf("error during quality control: %v", err)
	}

	return qualityControlledCompletion, nil
}
