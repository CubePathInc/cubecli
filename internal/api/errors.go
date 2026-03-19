package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return fmt.Sprintf("API error: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

func parseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{StatusCode: resp.StatusCode}
	}

	var result struct {
		Detail interface{} `json:"detail"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Detail != nil {
		switch v := result.Detail.(type) {
		case string:
			return &APIError{StatusCode: resp.StatusCode, Detail: v}
		default:
			detailBytes, _ := json.Marshal(v)
			return &APIError{StatusCode: resp.StatusCode, Detail: string(detailBytes)}
		}
	}

	if len(body) > 0 {
		return &APIError{StatusCode: resp.StatusCode, Detail: string(body)}
	}

	return &APIError{StatusCode: resp.StatusCode}
}
