package objectstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// metricsRanges maps the --range values to the GraphQL TimeRange enum.
var metricsRanges = map[string]string{
	"1h":  "H1",
	"3h":  "H3",
	"6h":  "H6",
	"12h": "H12",
	"24h": "H24",
	"3d":  "D3",
	"7d":  "D7",
	"30d": "D30",
}

// metricsParts are the chart groups of a bucket, in display order.
var metricsParts = []string{"storage", "traffic", "responses"}

type metricsPoint struct {
	TS    int64   `json:"ts"`
	Value float64 `json:"value"`
}

type metricsSeries struct {
	Name   string         `json:"name"`
	Unit   string         `json:"unit"`
	Points []metricsPoint `json:"points"`
}

type metricsResult struct {
	Start  int64           `json:"start"`
	End    int64           `json:"end"`
	Step   int             `json:"step"`
	Series []metricsSeries `json:"series"`
}

// parseMetricsParts validates a comma separated --part value; empty means every part.
func parseMetricsParts(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return metricsParts, nil
	}
	wanted := map[string]bool{}
	for _, p := range strings.Split(value, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		known := false
		for _, name := range metricsParts {
			if p == name {
				known = true
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown part %q: use storage, traffic or responses", p)
		}
		wanted[p] = true
	}
	parts := []string{}
	for _, name := range metricsParts {
		if wanted[name] {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return metricsParts, nil
	}
	return parts, nil
}

// bucketMetricsQuery asks only for the requested parts of objectStorageBucket.
func bucketMetricsQuery(parts []string) string {
	const result = "{ start end step series { name unit points { ts value } } }"
	var b strings.Builder
	b.WriteString("query($uuid: ID!, $range: TimeRange!) { objectStorageBucket(uuid: $uuid) { uuid name")
	for _, p := range parts {
		if p == "storage" {
			b.WriteString(" storageMeasuredAt")
		}
		b.WriteString(fmt.Sprintf(" %s(range: $range) %s", p, result))
	}
	b.WriteString(" } }")
	return b.String()
}

// formatMetricValue formats a value by its GraphQL unit.
func formatMetricValue(unit string, v float64) string {
	if unit == "BYTES" {
		return formatBytes(int64(v))
	}
	return formatCount(int64(v))
}

func bucketMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics <bucket>",
		Short: "Show a bucket's stored size, traffic and responses over a time range",
		Long: `Show the charts of a bucket: stored size and objects (hourly, the value billing
uses), billable traffic (egress, CDN, ingress, class A, class B and free requests of
the project's keys) and every response by status class.

Traffic and responses are totals per step, not rates; the table shows their sum over
the range and the stored size its latest value. --json prints every point.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			rangeFlag, _ := cmd.Flags().GetString("range")
			timeRange, ok := metricsRanges[strings.ToLower(strings.TrimSpace(rangeFlag))]
			if !ok {
				return fmt.Errorf("invalid range %q: use 1h, 3h, 6h, 12h, 24h, 3d, 7d or 30d", rangeFlag)
			}
			partFlag, _ := cmd.Flags().GetString("part")
			parts, err := parseMetricsParts(partFlag)
			if err != nil {
				return err
			}

			s := output.NewSpinner("Fetching bucket metrics...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var data json.RawMessage
			if err == nil {
				data, err = client.GraphQL(bucketMetricsQuery(parts), map[string]interface{}{"uuid": uuid, "range": timeRange})
			}
			s.Stop()
			var apiErr *api.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized {
				return fmt.Errorf("%s (bucket metrics are read through the GraphQL API, which does not accept browser logins yet: use an API token, for example CUBE_API_TOKEN=<token> cubecli s3 bucket metrics %s)", apiErr.Detail, args[0])
			}
			if err != nil {
				return err
			}

			var result struct {
				Bucket map[string]json.RawMessage `json:"objectStorageBucket"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			if result.Bucket == nil {
				return fmt.Errorf("bucket %q not found", args[0])
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(result.Bucket)
			}

			var name string
			_ = json.Unmarshal(result.Bucket["name"], &name)
			t := output.NewTable(fmt.Sprintf("Bucket %s, last %s", name, strings.ToLower(rangeFlag)), []string{"Part", "Series", "Value", "Step"})
			for _, p := range parts {
				var res metricsResult
				if err := json.Unmarshal(result.Bucket[p], &res); err != nil {
					return fmt.Errorf("failed to parse %s: %w", p, err)
				}
				for _, series := range res.Series {
					value := "-"
					if len(series.Points) > 0 {
						if p == "storage" {
							value = formatMetricValue(series.Unit, series.Points[len(series.Points)-1].Value) + " (latest)"
						} else {
							var sum float64
							for _, point := range series.Points {
								sum += point.Value
							}
							value = formatMetricValue(series.Unit, sum)
						}
					}
					t.AddRow(p, series.Name, value, (time.Duration(res.Step) * time.Second).String())
				}
			}
			t.Render()

			var measuredAt *int64
			if raw, ok := result.Bucket["storageMeasuredAt"]; ok {
				_ = json.Unmarshal(raw, &measuredAt)
				if measuredAt != nil {
					fmt.Printf("\nSize measured at %s\n", time.Unix(*measuredAt, 0).UTC().Format("2006-01-02 15:04 UTC"))
				} else {
					fmt.Println("\nSize not measured yet")
				}
			}
			return nil
		},
	}
	cmd.Flags().String("range", "24h", "Time range: 1h, 3h, 6h, 12h, 24h, 3d, 7d or 30d")
	cmd.Flags().String("part", "", "Comma separated parts: storage, traffic, responses (default: all)")
	return cmd
}
