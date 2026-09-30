package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/CubePathInc/cubecli/internal/version"
)

// TokenSource supplies the bearer token. Refresh is called once when the API
// answers 401 to a request sent with `stale`.
type TokenSource interface {
	Token() (string, error)
	Refresh(stale string) (string, error)
}

// StaticToken is a TokenSource for API tokens, which cannot be refreshed.
type StaticToken string

func (t StaticToken) Token() (string, error) { return string(t), nil }

func (t StaticToken) Refresh(string) (string, error) { return "", errNotRefreshable }

var errNotRefreshable = errors.New("token cannot be refreshed")

type Client struct {
	BaseURL    string
	Auth       TokenSource
	HTTPClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return NewClientWithAuth(baseURL, StaticToken(token))
}

func NewClientWithAuth(baseURL string, auth TokenSource) *Client {
	return &Client{
		BaseURL: baseURL,
		Auth:    auth,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (json.RawMessage, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}
	return c.doRaw(method, path, "application/json", data)
}

func (c *Client) doRaw(method, path, contentType string, data []byte) (json.RawMessage, error) {
	token, err := c.Auth.Token()
	if err != nil {
		return nil, err
	}
	resp, err := c.send(method, path, contentType, data, token)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		// An OAuth access token may have been revoked or expired early: refresh
		// once and retry. API tokens are not refreshable and keep the 401.
		if fresh, rerr := c.Auth.Refresh(token); rerr == nil {
			resp.Body.Close()
			if resp, err = c.send(method, path, contentType, data, fresh); err != nil {
				return nil, err
			}
		} else if !errors.Is(rerr, errNotRefreshable) {
			resp.Body.Close()
			return nil, rerr
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseErrorResponse(resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if len(respBody) == 0 {
		return json.RawMessage("{}"), nil
	}

	return json.RawMessage(respBody), nil
}

func (c *Client) send(method, path, contentType string, data []byte, token string) (*http.Response, error) {
	var reqBody io.Reader
	if data != nil {
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", fmt.Sprintf("CubeCLI/%s", version.Version))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}
	return resp, nil
}

func (c *Client) Get(path string) (json.RawMessage, error) {
	return c.doRequest(http.MethodGet, path, nil)
}

func (c *Client) Post(path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest(http.MethodPost, path, body)
}

func (c *Client) Put(path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest(http.MethodPut, path, body)
}

func (c *Client) Patch(path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest(http.MethodPatch, path, body)
}

func (c *Client) Delete(path string) (json.RawMessage, error) {
	return c.doRequest(http.MethodDelete, path, nil)
}

// PostFile uploads one file as multipart/form-data under the form field `field`.
func (c *Client) PostFile(path, field, fileName string, content []byte) (json.RawMessage, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to build upload: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, fmt.Errorf("failed to build upload: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to build upload: %w", err)
	}
	return c.doRaw(http.MethodPost, path, w.FormDataContentType(), buf.Bytes())
}
