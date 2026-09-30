package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGraphQLReturnsData(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"data":{"baremetal":{"id":"9"}}}`))
	}))
	defer srv.Close()

	data, err := NewClient(srv.URL, "t").GraphQL("query($id: ID!) { baremetal(id: $id) { id } }", map[string]interface{}{"id": "9"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"baremetal":{"id":"9"}}` {
		t.Fatalf("data %s", data)
	}
	if vars, _ := got["variables"].(map[string]interface{}); vars["id"] != "9" {
		t.Fatalf("body %v", got)
	}
}

func TestGraphQLNotFoundIs404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"baremetal":null},"errors":[{"message":"Resource not found.","extensions":{"code":"NOT_FOUND"}}]}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "t").GraphQL("{ x }", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound || apiErr.Detail != "Resource not found." {
		t.Fatalf("got %v", err)
	}
}
