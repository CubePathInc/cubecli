package vps

import (
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestBackupListShowsSnapshot(t *testing.T) {
	resp := `{"backups": [
	  {"id": 11, "backup_type": "manual", "status": "completed", "progress": 100, "size_gb": 3.5, "notes": "", "created_at": "2026-10-07", "snapshot_uuid": "6f1c-aaaa"},
	  {"id": 12, "backup_type": "auto", "status": "completed", "progress": 100, "size_gb": 3.6, "notes": "", "created_at": "2026-10-08", "snapshot_uuid": null}
	]}`
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(string, string) (int, string) { return 200, resp }, "backup", "list", "345")
	if err != nil {
		t.Fatal(err)
	}
	if cmdtest.Last(reqs).Path != "/vps/345/backups" {
		t.Fatalf("got %s", cmdtest.Last(reqs).Path)
	}
	if !strings.Contains(out, "Snapshot") || !strings.Contains(out, "6f1c-aaaa") {
		t.Fatalf("snapshot column missing:\n%s", out)
	}
	if strings.Count(out, "6f1c-aaaa") != 1 {
		t.Fatalf("only the converted backup should carry a snapshot:\n%s", out)
	}
}

func TestShowSnapshotFields(t *testing.T) {
	projects := func(extra string) string {
		return `[{"project": {"id": 7, "name": "prod"}, "vps": [{"id": 345, "name": "web", "status": "active",
		  "plan": {"plan_name": "gp.small"}, "template": {"os_name": "Ubuntu 24"}, "location": {"location_name": "eu-bcn-1"}` + extra + `}]}]`
	}

	out, _, err := cmdtest.Run(t, NewCmd(), func(string, string) (int, string) {
		return 200, projects(`, "source_snapshot_uuid": "6f1c-aaaa", "deploy_health": "degraded"`)
	}, "show", "345")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "From snapshot") || !strings.Contains(out, "6f1c-aaaa") ||
		!strings.Contains(out, "Deploy health") || !strings.Contains(out, "degraded") {
		t.Fatalf("snapshot rows missing:\n%s", out)
	}

	out, _, err = cmdtest.Run(t, NewCmd(), func(string, string) (int, string) {
		return 200, projects(`, "source_snapshot_uuid": null, "deploy_health": null`)
	}, "show", "345")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "From snapshot") || strings.Contains(out, "Deploy health") {
		t.Fatalf("snapshot rows should be hidden when null:\n%s", out)
	}
}
