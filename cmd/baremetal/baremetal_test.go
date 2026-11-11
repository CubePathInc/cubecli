package baremetal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type recorded struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

// run executes `baremetal <args>` against a fake API and returns the requests it sent.
func run(t *testing.T, graphql string, args ...string) ([]recorded, error) {
	t.Helper()
	var mu sync.Mutex
	var reqs []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		mu.Lock()
		reqs = append(reqs, recorded{r.Method, r.URL.Path, body})
		mu.Unlock()
		switch {
		case r.URL.Path == "/graphql":
			_, _ = w.Write([]byte(graphql))
		case r.URL.Path == "/projects/":
			_, _ = w.Write([]byte(`[{"project":{"id":1,"name":"p"},"baremetals":[{"id":"` + deployingUUID + `","uuid":"` + deployingUUID + `","status":"deploying"},{"id":"` + testUUID + `","uuid":"` + testUUID + `","status":"active"}]}]`))
		default:
			_, _ = w.Write([]byte(`{"detail":"ok"}`))
		}
	}))
	defer srv.Close()

	root := &cobra.Command{Use: "cubecli", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(NewCmd())
	root.SetArgs(append([]string{"baremetal"}, args...))
	ctx := context.WithValue(context.Background(), cmdutil.ClientKey, api.NewClient(srv.URL, "tok"))

	oldStdout, oldStderr := os.Stdout, os.Stderr
	devnull, _ := os.Open(os.DevNull)
	os.Stdout, os.Stderr = devnull, devnull
	err := root.ExecuteContext(ctx)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = devnull.Close()
	return reqs, err
}

func TestSensorsReadsGraphQL(t *testing.T) {
	reqs, err := run(t, `{"data":{"baremetal":{"sensors":{"ipmiAvailable":true,"powerOn":true,"lastSeen":100,"temperatures":[{"name":"CPU","value":41,"unit":"CELSIUS"}],"fans":[]}}}}`, "sensors", testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != http.MethodPost || reqs[0].Path != "/graphql" {
		t.Fatalf("got %+v", reqs)
	}
	if vars, _ := reqs[0].Body["variables"].(map[string]interface{}); vars["id"] != testUUID {
		t.Fatalf("body %v", reqs[0].Body)
	}
}

func TestSensorsUnknownServer(t *testing.T) {
	_, err := run(t, `{"data":{"baremetal":null},"errors":[{"message":"Resource not found.","extensions":{"code":"NOT_FOUND"}}]}`, "sensors", testUUID)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestSensorsOAuthLoginHint(t *testing.T) {
	_, err := run(t, `{"data":{"baremetal":null},"errors":[{"message":"Authentication required: send the session cookie or an API token.","extensions":{"code":"UNAUTHENTICATED"}}]}`, "sensors", testUUID)
	if err == nil || !strings.Contains(err.Error(), "CUBE_API_TOKEN") {
		t.Fatalf("got %v", err)
	}
}

func TestReinstallStatusFromServerStatus(t *testing.T) {
	reqs, err := run(t, "", "reinstall", "status", deployingUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Path != "/projects/" {
		t.Fatalf("got %+v", reqs)
	}
	if _, err := run(t, "", "reinstall", "status", "aaaaaaaa-0000-4000-8000-000000000000"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestReinstallCancel(t *testing.T) {
	reqs, err := run(t, "", "reinstall", "cancel", testUUID, "--force")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Method != http.MethodDelete || reqs[0].Path != "/baremetal/"+testUUID+"/reinstall" {
		t.Fatalf("got %+v", reqs)
	}
}
