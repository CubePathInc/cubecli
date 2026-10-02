package objectstorage

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestEventsDestinationCreateWebhookShowsSecretOnce(t *testing.T) {
	out, reqs, err := run(t, "s3", "events", "destination", "create", "--name", "uploads-hook", "--webhook", "https://example.com/hooks", "--format", "s3")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/event-destinations" {
		t.Fatalf("got %+v", req)
	}
	want := map[string]interface{}{"name": "uploads-hook", "type": "webhook", "url": "https://example.com/hooks", "payload_format": "s3"}
	if !reflect.DeepEqual(req.Body, want) {
		t.Fatalf("body %v", req.Body)
	}
	if !strings.Contains(out, "whsec_S3cretS3cretS3cretS3cretS3cr") || !strings.Contains(out, "https://example.com/***") {
		t.Fatalf("stdout:\n%s", out)
	}
}

func TestEventsDestinationCreateChannel(t *testing.T) {
	_, reqs, err := run(t, "s3", "events", "destination", "create", "--name", "ops", "--channel", "n1")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"name": "ops", "type": "notificator", "notificator_id": "n1"}
	if b := last(reqs).Body; !reflect.DeepEqual(b, want) {
		t.Fatalf("body %v", b)
	}
}

func TestEventsDestinationCreateNeedsOneTarget(t *testing.T) {
	if _, _, err := run(t, "s3", "events", "destination", "create", "--name", "x"); err == nil {
		t.Fatal("expected an error without --webhook or --channel")
	}
	if _, _, err := run(t, "s3", "events", "destination", "create", "--name", "x", "--webhook", "https://a", "--format", "xml"); err == nil {
		t.Fatal("expected an error for --format xml")
	}
}

func TestEventsDestinationListDoesNotShowSecret(t *testing.T) {
	out, _, err := run(t, "s3", "events", "destination", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "uploads-hook") || strings.Contains(out, "whsec_") {
		t.Fatalf("stdout:\n%s", out)
	}
}

func TestEventsDestinationRoutesByName(t *testing.T) {
	base := "/object-storage/event-destinations/" + destUUID
	cases := []struct {
		args   []string
		method string
		path   string
	}{
		{[]string{"get", "uploads-hook"}, http.MethodGet, base},
		{[]string{"update", "uploads-hook", "--disable"}, http.MethodPatch, base},
		{[]string{"delete", "uploads-hook", "--force"}, http.MethodDelete, base},
		{[]string{"rotate-secret", destUUID, "--force"}, http.MethodPost, base + "/rotate-secret"},
		{[]string{"test", "uploads-hook"}, http.MethodPost, base + "/test"},
		{[]string{"deliveries", "uploads-hook", "--status", "failed", "--limit", "10"}, http.MethodGet, base + "/deliveries?limit=10&status=failed"},
	}
	for _, c := range cases {
		out, reqs, err := run(t, append([]string{"s3", "events", "destination"}, c.args...)...)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if req := last(reqs); req.Method != c.method || req.Path != c.path {
			t.Fatalf("%v: got %+v", c.args, req)
		}
		if c.args[0] == "update" && last(reqs).Body["enabled"] != false {
			t.Fatalf("update body %v", last(reqs).Body)
		}
		if c.args[0] == "rotate-secret" && !strings.Contains(out, "whsec_S3cretS3cretS3cretS3cretS3cr") {
			t.Fatalf("rotate stdout:\n%s", out)
		}
		if c.args[0] == "deliveries" && (!strings.Contains(out, "incoming/a.jpg") || !strings.Contains(out, "500")) {
			t.Fatalf("deliveries stdout:\n%s", out)
		}
	}
}

func TestEventsDestinationUpdateNeedsAField(t *testing.T) {
	if _, _, err := run(t, "s3", "events", "destination", "update", "uploads-hook"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestEventsRuleCreate(t *testing.T) {
	out, reqs, err := run(t, "s3", "events", "rule", "create", "--bucket", "photos", "--destination", "uploads-hook",
		"--events", "created,object.removed,created", "--prefix", "incoming/", "--suffix", ".jpg")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/buckets/"+bucketUUID+"/event-rules" {
		t.Fatalf("got %+v", req)
	}
	want := map[string]interface{}{
		"name": "on-created-removed", "destination_uuid": destUUID,
		"events": []interface{}{"object.created", "object.removed"},
		"prefix": "incoming/", "suffix": ".jpg", "enabled": true,
	}
	if !reflect.DeepEqual(req.Body, want) {
		t.Fatalf("body %v", req.Body)
	}
	if !strings.Contains(out, "pending") {
		t.Fatalf("stdout:\n%s", out)
	}
}

func TestEventsRuleCreateRejectsUnknownEvent(t *testing.T) {
	if _, _, err := run(t, "s3", "events", "rule", "create", "--bucket", "photos", "--destination", "uploads-hook", "--events", "copied"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestEventsRuleListUpdateDelete(t *testing.T) {
	base := "/object-storage/buckets/" + bucketUUID + "/event-rules"
	out, reqs, err := run(t, "s3", "events", "rule", "list", "--bucket", "photos")
	if err != nil || last(reqs).Path != base || !strings.Contains(out, "on-created") {
		t.Fatalf("list: %v %+v\n%s", err, last(reqs), out)
	}

	_, reqs, err = run(t, "s3", "events", "rule", "update", "on-created", "--bucket", "photos", "--events", "tagging", "--suffix", "", "--enable")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	want := map[string]interface{}{"events": []interface{}{"object.tagging"}, "suffix": "", "enabled": true}
	if req.Method != http.MethodPatch || req.Path != base+"/"+ruleUUID || !reflect.DeepEqual(req.Body, want) {
		t.Fatalf("update %+v", req)
	}

	_, reqs, err = run(t, "s3", "events", "rule", "delete", ruleUUID, "--bucket", "photos", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodDelete || req.Path != base+"/"+ruleUUID {
		t.Fatalf("delete %+v", req)
	}
}

func TestParseDeliveriesAcceptsWrappedList(t *testing.T) {
	rows, err := parseDeliveries([]byte(`{"deliveries":[{"status":"success","attempt":1}]}`))
	if err != nil || len(rows) != 1 || rows[0].Status != "success" {
		t.Fatalf("%v %+v", err, rows)
	}
}
