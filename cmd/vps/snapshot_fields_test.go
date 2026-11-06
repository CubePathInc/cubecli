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

func TestShowDisk(t *testing.T) {
	show := func(extra string) string {
		out, _, err := cmdtest.Run(t, NewCmd(), func(string, string) (int, string) {
			return 200, `[{"project": {"id": 7, "name": "prod"}, "vps": [{"id": 345, "name": "web", "status": "active",
			  "plan": {"plan_name": "gp.small", "storage": 80}, "template": {"os_name": "Ubuntu 24"}, "location": {"location_name": "eu-bcn-1"}` + extra + `}]}]`
		}, "show", "345")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := show(`, "disk_gb": 40`); !strings.Contains(out, "40 GB (plan includes 80 GB)") {
		t.Fatalf("smaller disk not explained:\n%s", out)
	}
	if out := show(`, "disk_gb": 80`); !strings.Contains(out, "80 GB") || strings.Contains(out, "plan includes") {
		t.Fatalf("full disk:\n%s", out)
	}
	if out := show(``); !strings.Contains(out, "80 GB") || strings.Contains(out, "plan includes") {
		t.Fatalf("fallback to plan storage:\n%s", out)
	}
}

func TestResizeGrowDisk(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "resize", "345", "--plan", "gp.big", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if got := cmdtest.Last(reqs).Path; got != "/vps/resize/vps_id/345/resize_plan/gp.big" {
		t.Fatalf("grow_disk must not be sent by default: %s", got)
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "resize", "345", "--plan", "gp.big", "--grow-disk", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if got := cmdtest.Last(reqs).Path; got != "/vps/resize/vps_id/345/resize_plan/gp.big?grow_disk=true" {
		t.Fatalf("got %s", got)
	}
}
