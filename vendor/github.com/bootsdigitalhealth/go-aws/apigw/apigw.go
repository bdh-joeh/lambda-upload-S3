package apigw

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

const (
	origin = "*"
)

// ErrorResponse returns an API Gateway response with a standard body
func ErrorResponse(code int, msg string) events.APIGatewayProxyResponse {
	type ErrorBody struct {
		Code         int    `json:"code"`
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	err := ErrorBody{
		Code:         code,
		Status:       http.StatusText(code),
		ErrorMessage: msg,
	}
	body, _ := json.Marshal(err)
	return events.APIGatewayProxyResponse{
		StatusCode: code,
		Body:       string(body),
		Headers: map[string]string{
			"Access-Control-Allow-Origin": origin,
		},
	}
}

// CORSErrorResponse returns an API Gateway response with a standard body and CORS headers`
func CORSErrorResponse(code int, msg string, request events.APIGatewayProxyRequest, corsHelper *CorsHelper) events.APIGatewayProxyResponse {
	type ErrorBody struct {
		Code         int    `json:"code"`
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	}
	err := ErrorBody{
		Code:         code,
		Status:       http.StatusText(code),
		ErrorMessage: msg,
	}
	body, _ := json.Marshal(err)
	response := corsHelper.GetCORSResponse(request)
	response.StatusCode = code
	response.Body = string(body)
	return response
}

// CorsHelper is a struct for handling CORS
type CorsHelper struct {
	Enabled          bool
	AllowHeaders     string
	AllowMethods     string
	AllowCredentials string
	AllowOrigins     []string
}

// NewCorsHelper returns a new CorsHelper struct
func NewCorsHelper(enabled, allowHeaders, allowMethods, allowCredentials, allowOrigins string) *CorsHelper {
	return &CorsHelper{
		Enabled:          enabled == "true",
		AllowHeaders:     allowHeaders,
		AllowMethods:     allowMethods,
		AllowCredentials: allowCredentials,
		AllowOrigins:     strings.Split(allowOrigins, ","),
	}
}

// GetCORSResponse returns an API Gateway response with CORS headers
func (c *CorsHelper) GetCORSResponse(request events.APIGatewayProxyRequest) events.APIGatewayProxyResponse {
	origin := c.GetOrigin(request)
	event := events.APIGatewayProxyResponse{}
	if c.Enabled {
		headers := make(map[string]string)
		if len(c.AllowOrigins) == 1 && c.AllowOrigins[0] == "*" {
			headers["Access-Control-Allow-Origin"] = "*"
		} else if c.originIsValid(origin) {
			headers["Access-Control-Allow-Origin"] = origin
		}
		if c.AllowHeaders != "" {
			headers["Access-Control-Allow-Headers"] = c.AllowHeaders
		}
		if c.AllowMethods != "" {
			headers["Access-Control-Allow-Methods"] = c.AllowMethods
		}
		if c.AllowCredentials != "" {
			headers["Access-Control-Allow-Credentials"] = c.AllowCredentials
		}
		event.Headers = headers
	}
	return event
}

// GetOrigin returns the origin from the request headers
func (c *CorsHelper) GetOrigin(request events.APIGatewayProxyRequest) string {
	origin, ok := request.Headers["Origin"]
	if ok {
		return origin
	}
	origin, ok = request.Headers["origin"]
	if ok {
		return origin
	}
	return ""
}

// originIsValid returns true if the origin is in the list of allowed origins
func (c *CorsHelper) originIsValid(origin string) bool {
	for _, o := range c.AllowOrigins {
		if o == origin {
			return true
		}
	}
	return false
}
