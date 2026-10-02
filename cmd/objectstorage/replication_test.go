package objectstorage

import (
	"net/http"
	"strings"
	"testing"
)

const replicationJSON = `{"uuid":"` + replUUID + `","status":"active","pause_reason":null,"direction":"outgoing",
	"source":{"bucket_uuid":"` + bucketUUID + `","bucket_name":"photos","project_id":12,"organization_name":"Acme","same_organization":true},
	"destination":{"type":"external","provider":"aws","endpoint":"s3.eu-west-1.amazonaws.com","region":"eu-west-1","bucket":"acme-backup","path_style":"auto","access_key_id":"****WXYZ"},
	"rules":{"enabled":true,"prefix":"img/","tags":[],"delete_marker_replication":true,"delete_replication":false,"existing_objects":true},
	"health":"lagging","health_reason":null,"health_checked_at":"2026-10-02T10:00:00",
	"backfill":{"status":"completed","started_at":"2026-10-02T09:00:00","finished_at":"2026-10-02T09:40:00","objects":120034,"bytes":53687091200,"failed_objects":0},
	"error_message":null,"created_at":"2026-10-02T08:59:30","active_at":"2026-10-02T09:00:05"}`

func TestReplicationListTableAndFilters(t *testing.T) {
	out, reqs, err := run(t, "s3", "replication", "list", "--direction", "outgoing", "--bucket", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/replications?bucket_uuid="+bucketUUID+"&direction=outgoing" {
		t.Fatalf("path %s", got)
	}
	for _, want := range []string{"acme-backup on s3.eu-west-1.amazonaws.com", "prefix img/, delete markers", "lagging", "completed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout misses %q:\n%s", want, out)
		}
	}
	if _, _, err := run(t, "s3", "replication", "list", "--direction", "sideways"); err == nil {
		t.Fatal("expected an error for a bad --direction")
	}
}

func TestReplicationGetBySourceBucketName(t *testing.T) {
	out, reqs, err := run(t, "s3", "replication", "get", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/replications/"+replUUID {
		t.Fatalf("path %s", got)
	}
	for _, want := range []string{"****WXYZ", "120,034 objects, 50.00 GiB", "1.0 MiB", "5.0 MiB"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout misses %q:\n%s", want, out)
		}
	}
	if _, _, err := run(t, "s3", "replication", "get", "nope"); err == nil || !strings.Contains(err.Error(), `no replication found for "nope"`) {
		t.Fatalf("err %v", err)
	}
}

func TestReplicationCreateCubepath(t *testing.T) {
	_, reqs, err := run(t, "s3", "replication", "create", "photos", "--dest-bucket", "backups", "--tag", "class=archive", "--no-existing-objects")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/replications" {
		t.Fatalf("got %+v", req)
	}
	dest := req.Body["destination"].(map[string]interface{})
	if req.Body["source_bucket_uuid"] != bucketUUID || dest["type"] != "cubepath" || dest["bucket_uuid"] != otherUUID {
		t.Fatalf("body %v", req.Body)
	}
	if _, ok := dest["grant_token"]; ok {
		t.Fatalf("no grant token was given: %v", dest)
	}
	if req.Body["existing_objects"] != false || req.Body["delete_marker_replication"] != false {
		t.Fatalf("body %v", req.Body)
	}
	tags := req.Body["tags"].([]interface{})
	if tag := tags[0].(map[string]interface{}); tag["key"] != "class" || tag["value"] != "archive" {
		t.Fatalf("tags %v", tags)
	}
	if _, has := req.Body["prefix"]; has {
		t.Fatalf("prefix sent without --prefix: %v", req.Body)
	}
}

func TestReplicationCreateOtherOrganizationWithGrant(t *testing.T) {
	other := "dddddddd-eeee-4fff-8000-111111111111"
	_, reqs, err := run(t, "s3", "replication", "create", "photos", "--dest-bucket", other, "--grant-token", "cprg_AbCd")
	if err != nil {
		t.Fatal(err)
	}
	dest := last(reqs).Body["destination"].(map[string]interface{})
	if dest["bucket_uuid"] != other || dest["grant_token"] != "cprg_AbCd" {
		t.Fatalf("destination %v", dest)
	}
}

func TestReplicationCreateExternalSecretFromStdin(t *testing.T) {
	t.Setenv(replSecretEnv, "from-env")
	_, reqs, err := runWithStdin(t, "s3cr3t\n", "s3", "replication", "create", "photos", "--external", "--provider", "aws",
		"--endpoint", "s3.eu-west-1.amazonaws.com", "--region", "eu-west-1", "--bucket", "acme-backup",
		"--access-key", "AKIAEXAMPLE", "--secret-key-stdin", "--prefix", "img/")
	if err != nil {
		t.Fatal(err)
	}
	body := last(reqs).Body
	dest := body["destination"].(map[string]interface{})
	want := map[string]interface{}{"type": "external", "provider": "aws", "endpoint": "s3.eu-west-1.amazonaws.com", "region": "eu-west-1",
		"bucket": "acme-backup", "path_style": "auto", "access_key_id": "AKIAEXAMPLE", "secret_access_key": "s3cr3t"}
	for k, v := range want {
		if dest[k] != v {
			t.Fatalf("destination[%s] = %v, want %v", k, dest[k], v)
		}
	}
	if _, has := dest["bucket_uuid"]; has || body["prefix"] != "img/" || body["existing_objects"] != true {
		t.Fatalf("body %v", body)
	}
}

func TestReplicationCreateExternalSecretFromEnv(t *testing.T) {
	t.Setenv(replSecretEnv, "from-env")
	_, reqs, err := runWithStdin(t, "ignored\n", "s3", "replication", "create", "photos", "--external",
		"--endpoint", "s3.wasabisys.com", "--region", "eu-central-1", "--bucket", "acme-backup", "--access-key", "AK")
	if err != nil {
		t.Fatal(err)
	}
	if dest := last(reqs).Body["destination"].(map[string]interface{}); dest["secret_access_key"] != "from-env" || dest["provider"] != "other" {
		t.Fatalf("destination %v", dest)
	}
}

func TestReplicationCreateRejectsBadInput(t *testing.T) {
	t.Setenv(replSecretEnv, "")
	cases := [][]string{
		{"photos"}, // no destination
		{"photos", "--dest-bucket", "backups", "--endpoint", "s3.example.org"},
		{"photos", "--external", "--dest-bucket", "backups"},
		{"photos", "--external", "--endpoint", "s3.example.org", "--region", "eu", "--bucket", "b-1"}, // no access key
		{"photos", "--external", "--endpoint", "https://s3.example.org", "--region", "eu", "--bucket", "b-1", "--access-key", "AK"},
		{"photos", "--external", "--endpoint", "s3.example.org", "--region", "eu", "--bucket", "b-1", "--access-key", "AK", "--provider", "gcs"},
		{"photos", "--external", "--endpoint", "s3.example.org", "--region", "eu", "--bucket", "b-1", "--access-key", "AK"}, // empty secret
		{"photos", "--dest-bucket", "backups", "--prefix", "a/", "--tag", "k=v"},
		{"photos", "--dest-bucket", "backups", "--tag", "k=v", "--delete-markers"},
		{"photos", "--dest-bucket", "backups", "--tag", "k=v", "--tag", "k=w"},
	}
	for _, args := range cases {
		_, reqs, err := runWithStdin(t, "", append([]string{"s3", "replication", "create"}, args...)...)
		if err == nil {
			t.Fatalf("%v: expected an error", args)
		}
		for _, r := range reqs {
			if r.Method == http.MethodPost {
				t.Fatalf("%v: sent %+v", args, r)
			}
		}
	}
}

func TestReplicationUpdate(t *testing.T) {
	_, reqs, err := run(t, "s3", "replication", "update", "photos", "--enabled=false", "--prefix", "docs/", "--deletes")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPatch || req.Path != "/object-storage/replications/"+replUUID {
		t.Fatalf("got %+v", req)
	}
	if req.Body["enabled"] != false || req.Body["prefix"] != "docs/" || req.Body["delete_replication"] != true {
		t.Fatalf("body %v", req.Body)
	}
	if v, has := req.Body["tags"]; !has || v != nil {
		t.Fatalf("a new prefix must drop the tags explicitly: %v", req.Body)
	}
	if _, has := req.Body["existing_objects"]; has {
		t.Fatalf("unchanged flags must not be sent: %v", req.Body)
	}

	_, reqs, err = run(t, "s3", "replication", "update", replUUID, "--clear-filter")
	if err != nil {
		t.Fatal(err)
	}
	body := last(reqs).Body
	if v, has := body["prefix"]; !has || v != nil {
		t.Fatalf("body %v", body)
	}

	if _, _, err := run(t, "s3", "replication", "update", "photos"); err == nil {
		t.Fatal("expected an error without changes")
	}
}

func TestReplicationUpdateRotateCredentials(t *testing.T) {
	t.Setenv(replSecretEnv, "")
	_, reqs, err := runWithStdin(t, "n3w\n", "s3", "replication", "update", "photos", "--rotate-credentials", "--access-key", "AKNEW", "--secret-key-stdin")
	if err != nil {
		t.Fatal(err)
	}
	dest := last(reqs).Body["destination"].(map[string]interface{})
	if dest["access_key_id"] != "AKNEW" || dest["secret_access_key"] != "n3w" {
		t.Fatalf("destination %v", dest)
	}
	if _, _, err := run(t, "s3", "replication", "update", "photos", "--access-key", "AKNEW"); err == nil {
		t.Fatal("--access-key without --rotate-credentials must fail")
	}
}

func TestReplicationDeleteResyncRevoke(t *testing.T) {
	_, reqs, err := run(t, "s3", "replication", "delete", "photos", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodDelete || req.Path != "/object-storage/replications/"+replUUID {
		t.Fatalf("got %+v", req)
	}

	_, reqs, err = run(t, "s3", "replication", "resync", "photos", "--older-than-days", "3")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodPost || req.Path != "/object-storage/replications/"+replUUID+"/resync" || req.Body["older_than_days"] != float64(3) {
		t.Fatalf("got %+v", req)
	}
	_, reqs, err = run(t, "s3", "replication", "resync", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if _, has := last(reqs).Body["older_than_days"]; has {
		t.Fatalf("got %+v", last(reqs))
	}
	if _, _, err := run(t, "s3", "replication", "resync", "photos", "--older-than-days", "0"); err == nil {
		t.Fatal("expected an error for --older-than-days 0")
	}

	_, reqs, err = run(t, "s3", "replication", "revoke", replUUID, "--force")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodPost || req.Path != "/object-storage/replications/"+replUUID+"/revoke" {
		t.Fatalf("got %+v", req)
	}
	if _, _, err := run(t, "s3", "replication", "revoke", "photos", "--force"); err == nil {
		t.Fatal("revoke takes a uuid only")
	}
}

func TestReplicationDeleteWithoutConfirmationAborts(t *testing.T) {
	_, reqs, err := runWithStdin(t, "n\n", "s3", "replication", "delete", "photos")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range reqs {
		if r.Method == http.MethodDelete {
			t.Fatalf("deleted without confirmation: %+v", r)
		}
	}
}

func TestReplicationGrants(t *testing.T) {
	out, reqs, err := run(t, "s3", "replication", "grant", "create", "backups", "--note", "for Acme", "--expires-in-days", "3")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/buckets/"+otherUUID+"/replication-grants" ||
		req.Body["note"] != "for Acme" || req.Body["expires_in_days"] != float64(3) {
		t.Fatalf("got %+v", req)
	}
	if !strings.Contains(out, "cprg_AbCdEfGhIjKlMnOpQrStUvWxYz012345") {
		t.Fatalf("token not shown:\n%s", out)
	}
	if _, _, err := run(t, "s3", "replication", "grant", "create", "backups", "--expires-in-days", "31"); err == nil {
		t.Fatal("expected an error for 31 days")
	}

	out, _, err = run(t, "s3", "replication", "grant", "list", "backups")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cprg_AbCd...") || strings.Contains(out, "cprg_AbCdEf") {
		t.Fatalf("stdout:\n%s", out)
	}

	_, reqs, err = run(t, "s3", "replication", "grant", "delete", keyUUID, "--force")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodDelete || req.Path != "/object-storage/replication-grants/"+keyUUID {
		t.Fatalf("got %+v", req)
	}
}
