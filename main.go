package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
)

// Define your database schema here
type DatabaseSchema struct {
	// ...
}

func main() {
	// Load environment variables
	err := loadEnv()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize OpenAI client
	openai.APIKey = os.Getenv("OPEN_AI_API_KEY")

	// Initialize Gin router
	router := gin.Default()

	// Define the /human_query endpoint
	router.POST("/human_query", handleHumanQuery)

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	router.Run(":" + port)
}

func loadEnv() error {
	return nil // Replace with your .env loading logic
}

func getSchema() DatabaseSchema {
	// Your function to get the database schema
	return DatabaseSchema{} // Replace with your actual schema
}

func query(sqlQuery string) ([]map[string]interface{}, error) {
	// Your function to query the database
	return nil, nil // Replace with your actual database query logic
}

func humanQueryToSQL(humanQuery string) (string, error) {
	databaseSchema := getSchema()
	schemaJSON, err := json.Marshal(databaseSchema)
	if err != nil {
		return "", fmt.Errorf("error marshaling database schema: %w", err)
	}

	systemMessage := fmt.Sprintf(`
    Given the following schema, write a SQL query that retrieves the requested information. 
    Return the SQL query inside a JSON structure with the key "sql_query".
    <example>{
        "sql_query": "SELECT * FROM users WHERE age > 18;"
        "original_query": "Show me all users older than 18 years old."
    }
    </example>
    <schema>
    %s
    </schema>
    `, string(schemaJSON))

	client := openai.NewClient(openai.APIKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemMessage,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: humanQuery,
				},
			},
			ResponseFormat: openai.ChatCompletionResponseFormatJSONObject,
		},
	)
	if err != nil {
		return "", fmt.Errorf("error calling OpenAI API: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func buildAnswer(result []map[string]interface{}, humanQuery string) (string, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshaling result: %w", err)
	}

	systemMessage := fmt.Sprintf(`
    Given a users question and the SQL rows response from the database from which the user wants to get the answer,
    write a response to the user's question.
    <user_question> 
    %s
    </user_question>
    <sql_response>
    %s 
    </sql_response>
    `, humanQuery, string(resultJSON))

	client := openai.NewClient(openai.APIKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemMessage,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("error calling OpenAI API: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

type PostHumanQueryPayload struct {
	HumanQuery string `json:"human_query"`
}

type PostHumanQueryResponse struct {
	Answer string `json:"answer"`
}

func handleHumanQuery(c *gin.Context) {
	var payload PostHumanQueryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	sqlQuery, err := humanQueryToSQL(payload.HumanQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate SQL query"})
		return
	}

	var resultDict map[string]interface{}
	err = json.Unmarshal([]byte(sqlQuery), &resultDict)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse SQL query"})
		return
	}

	result, err := query(resultDict["sql_query"].(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query database"})
		return
	}

	answer, err := buildAnswer(result, payload.HumanQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate answer"})
		return
	}

	c.JSON(http.StatusOK, PostHumanQueryResponse{Answer: answer})
}