package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GraphQL runs a query against POST /graphql and returns its "data" object. Metrics and BMC
// sensors are only served through GraphQL. A GraphQL error comes back as an *APIError;
// NOT_FOUND maps to 404.
func (c *Client) GraphQL(query string, variables map[string]interface{}) (json.RawMessage, error) {
	raw, err := c.Post("/graphql", map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				Code string `json:"code"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		status := http.StatusBadRequest
		messages := make([]string, 0, len(envelope.Errors))
		for _, e := range envelope.Errors {
			messages = append(messages, e.Message)
			switch e.Extensions.Code {
			case "NOT_FOUND":
				status = http.StatusNotFound
			case "FORBIDDEN":
				status = http.StatusForbidden
			case "UNAUTHENTICATED":
				status = http.StatusUnauthorized
			}
		}
		return nil, &APIError{StatusCode: status, Detail: strings.Join(messages, "; ")}
	}
	return envelope.Data, nil
}
