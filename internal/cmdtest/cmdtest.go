// Package cmdtest runs a command group against a fake API in tests.
package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Request is one call the command sent.
type Request struct {
	Method      string
	Path        string // path plus raw query
	ContentType string
	Raw         []byte
	Body        interface{} // decoded JSON body, nil when empty or not JSON
}

// Obj returns the body as a JSON object.
func (r Request) Obj() map[string]interface{} {
	m, _ := r.Body.(map[string]interface{})
	return m
}

// Responder answers a request; returning "" answers {"detail":"ok"}.
type Responder func(method, path string) (status int, body string)

// Run executes `<group> <args...>` against a fake API and returns what it
// printed to stdout and the requests it sent.
func Run(t *testing.T, group *cobra.Command, respond Responder, args ...string) (string, []Request, error) {
	t.Helper()
	var mu sync.Mutex
	var reqs []Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body interface{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		mu.Lock()
		reqs = append(reqs, Request{r.Method, path, r.Header.Get("Content-Type"), raw, body})
		mu.Unlock()

		status, out := 0, ""
		if respond != nil {
			status, out = respond(r.Method, path)
		}
		if status == 0 {
			status = http.StatusOK
		}
		if out == "" {
			out = `{"detail":"ok"}`
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(out))
	}))
	defer srv.Close()

	root := &cobra.Command{Use: "cubecli", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(group)
	root.SetArgs(append([]string{group.Name()}, args...))
	ctx := context.WithValue(context.Background(), cmdutil.ClientKey, api.NewClient(srv.URL, "tok"))

	oldStdout, oldStderr, oldColor := os.Stdout, os.Stderr, color.Output
	r, w, _ := os.Pipe()
	os.Stdout = w
	color.Output = w
	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	err := root.ExecuteContext(ctx)
	_ = w.Close()
	os.Stdout, os.Stderr, color.Output = oldStdout, oldStderr, oldColor
	_ = devnull.Close()
	return <-done, reqs, err
}

// Last returns the last request, or a zero Request.
func Last(reqs []Request) Request {
	if len(reqs) == 0 {
		return Request{}
	}
	return reqs[len(reqs)-1]
}
