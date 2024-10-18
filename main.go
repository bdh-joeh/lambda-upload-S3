package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
    "errors"
    "log"
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootsdigitalhealth/go-aws/apigw"

	"github.com/aws/aws-lambda-go/events"

)

// S3Uploader is a wrapper for S3 client
type S3Uploader struct {
	client *s3.Client
	bucket string
}


func validateJSON(jsonData string) error {
    var temp interface{}
    
    // Unmarshal the JSON data into a generic interface
    if err := json.Unmarshal([]byte(jsonData), &temp); err != nil {
        return fmt.Errorf("invalid JSON format: %v", err)
    }

    // Ensure the top-level structure is either a JSON object or array
    switch temp.(type) {
    case map[string]interface{}:
        // Valid JSON object
    case []interface{}:
        // Valid JSON array
    default:
        return fmt.Errorf("invalid JSON: must be an object or array")
    }

    return nil
}


// NewS3Uploader initializes the S3 client
func NewS3Uploader(bucket string) (*S3Uploader, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-2"))
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	return &S3Uploader{client: client, bucket: bucket}, nil
}

// UploadJSON uploads the JSON string to the S3 bucket
func (u *S3Uploader) UploadJSON(key string, data string) error {
	_, err := u.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(data), // Use strings.NewReader here
		ContentType: aws.String("application/json"),
	})
	return err
}

func errorResponse(statusCode int, err error) (events.APIGatewayProxyResponse, error) {
	return apigw.ErrorResponse(statusCode, err.Error()), nil
}

func Handler(request events.APIGatewayProxyRequest, ctx context.Context, event json.RawMessage) (events.APIGatewayProxyResponse, error) {

	log.Printf("Handling request: %s\n", request.Resource)

	// check authorization
	if len(request.Headers["Authorization"]) == 0 {
		return errorResponse(http.StatusUnauthorized, errors.New("authentication token is missing"))
	}

	// get IP for audit
	ip, err := getIP(request)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, err)
	}

	// get the user session, refresh if valid
	session, statusCode, err := getAndRefreshSession(sessionsRedisClient, request)
	if err != nil {
		return errorResponse(statusCode, err)
	}

    // Convert incoming event (JSON) to string
    jsonData := string(event)

    // Validate the JSON structure
    if err := validateJSON(jsonData); err != nil {
        return "", err
    }

    // Create an S3 uploader instance
    bucketName := os.Getenv("BUCKET_NAME") // Use the S3 bucket name from environment variables
    uploader, err := NewS3Uploader(bucketName)
    if err != nil {
        return "", fmt.Errorf("failed to create S3 uploader: %v", err)
    }

    // Create a unique file name based on the current timestamp
    timestamp := time.Now().Format("2006-01-02_15-04-05")
    fileName := fmt.Sprintf("actions/user_action_%s.json", timestamp)

    // Upload the validated JSON string to S3
    if err = uploader.UploadJSON(fileName, jsonData); err != nil {
        return "", fmt.Errorf("failed to upload JSON to S3: %v", err)
    }

    return fmt.Sprintf("Data uploaded successfully to S3: %s/%s", bucketName, fileName), nil
}

func main() {
	lambda.Start(Handler)
}