package output

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// RenderSeries prints a {"metrics": {name: [[ts, value], ...]}} envelope as
// one row per metric with its latest value, min, max and sample count.
func RenderSeries(title string, resp json.RawMessage) error {
	var env struct {
		Metrics map[string][][2]float64 `json:"metrics"`
	}
	if err := json.Unmarshal(resp, &env); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	names := make([]string, 0, len(env.Metrics))
	for n := range env.Metrics {
		names = append(names, n)
	}
	sort.Strings(names)

	t := NewTable(title, []string{"Metric", "Latest", "Min", "Max", "Samples"})
	for _, n := range names {
		pts := env.Metrics[n]
		if len(pts) == 0 {
			t.AddRow(n, "-", "-", "-", "0")
			continue
		}
		lo, hi := pts[0][1], pts[0][1]
		for _, p := range pts {
			lo = min(lo, p[1])
			hi = max(hi, p[1])
		}
		t.AddRow(n, fmt.Sprintf("%g", pts[len(pts)-1][1]), fmt.Sprintf("%g", lo), fmt.Sprintf("%g", hi), strconv.Itoa(len(pts)))
	}
	t.Render()
	return nil
}
