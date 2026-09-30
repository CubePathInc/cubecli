package transcoder

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	tcCmd := &cobra.Command{
		Use:   "transcoder",
		Short: "Manage video transcoding jobs",
		Long: `Transcode videos from a URL or an S3-compatible bucket into files, HLS,
thumbnails or GIFs written to your own S3-compatible bucket. Secret keys are
stored encrypted and never returned.`,
	}

	tcCmd.AddCommand(
		listCmd(),
		showCmd(),
		createCmd(),
		batchCmd(),
		outputsCmd(),
		cancelCmd(),
	)
	return tcCmd
}

// job is the job representation returned by every job endpoint.
type job struct {
	UUID              string `json:"uuid"`
	Status            string `json:"status"`
	Progress          int    `json:"progress"`
	TotalSegments     int    `json:"total_segments"`
	CompletedSegments int    `json:"completed_segments"`
	BatchID           string `json:"batch_id"`
	Error             string `json:"error"`
	CreatedAt         string `json:"created_at"`
	Input             struct {
		Source string `json:"source"`
		URL    string `json:"url"`
		S3     *struct {
			Bucket string `json:"bucket"`
			Path   string `json:"path"`
		} `json:"s3"`
	} `json:"input"`
	Output struct {
		S3 struct {
			Bucket string `json:"bucket"`
			Path   string `json:"path"`
		} `json:"s3"`
	} `json:"output"`
	Spec struct {
		Outputs []struct {
			Type string `json:"type"`
		} `json:"outputs"`
	} `json:"spec"`
}

func (j job) source() string {
	if j.Input.Source == "s3" && j.Input.S3 != nil {
		return fmt.Sprintf("s3://%s/%s", j.Input.S3.Bucket, j.Input.S3.Path)
	}
	return j.Input.URL
}

func (j job) formats() string {
	types := make([]string, 0, len(j.Spec.Outputs))
	for _, o := range j.Spec.Outputs {
		types = append(types, o.Type)
	}
	return strings.Join(types, ", ")
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transcoding jobs (newest first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			q := url.Values{}
			if b, _ := cmd.Flags().GetString("batch"); b != "" {
				q.Set("batch_id", b)
			}
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			q.Set("limit", strconv.Itoa(limit))
			q.Set("offset", strconv.Itoa(offset))

			s := output.NewSpinner("Fetching jobs...")
			s.Start()
			resp, err := client.Get("/transcoder/jobs?" + q.Encode())
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				Jobs []job `json:"jobs"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Transcoding Jobs", []string{"UUID", "Status", "Progress", "Source", "Formats", "Created"})
			for _, j := range r.Jobs {
				t.AddRow(j.UUID, output.FormatStatus(j.Status), fmt.Sprintf("%d%%", j.Progress), j.source(), j.formats(), j.CreatedAt)
			}
			t.Render()
			if len(r.Jobs) == limit {
				output.PrintInfo(fmt.Sprintf("More jobs may exist: use --offset %d", offset+limit))
			}
			return nil
		},
	}
	cmd.Flags().String("batch", "", "Only jobs of this batch id")
	cmd.Flags().Int("limit", 100, "Jobs per page (1-500)")
	cmd.Flags().Int("offset", 0, "Jobs to skip")
	return cmd
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <job_uuid>",
		Short: "Show a transcoding job and its progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching job...")
			s.Start()
			resp, err := client.Get("/transcoder/jobs/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderJob("Transcoding Job", resp)
		},
	}
}

func renderJob(title string, resp json.RawMessage) error {
	var j job
	if err := json.Unmarshal(resp, &j); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"Field", "Value"})
	t.AddRow("UUID", j.UUID)
	t.AddRow("Status", output.FormatStatus(j.Status))
	t.AddRow("Progress", fmt.Sprintf("%d%% (%d/%d segments)", j.Progress, j.CompletedSegments, j.TotalSegments))
	t.AddRow("Source", j.source())
	t.AddRow("Destination", fmt.Sprintf("s3://%s/%s", j.Output.S3.Bucket, j.Output.S3.Path))
	t.AddRow("Formats", j.formats())
	if j.BatchID != "" {
		t.AddRow("Batch", j.BatchID)
	}
	if j.Error != "" {
		t.AddRow("Error", j.Error)
	}
	t.AddRow("Created", j.CreatedAt)
	t.Render()
	return nil
}

// --- create ---

// s3Flags reads the --<prefix>-bucket/-path/-endpoint/-region/-access-key/-secret-key flags.
func s3Flags(cmd *cobra.Command, prefix string) map[string]interface{} {
	s3 := map[string]interface{}{}
	for flag, field := range map[string]string{
		"bucket": "bucket", "path": "path", "endpoint": "endpoint",
		"region": "region", "access-key": "access_key", "secret-key": "secret_key",
	} {
		if v, _ := cmd.Flags().GetString(prefix + "-" + flag); v != "" {
			s3[field] = v
		}
	}
	return s3
}

func addS3Flags(cmd *cobra.Command, prefix, what string) {
	cmd.Flags().String(prefix+"-bucket", "", what+" bucket")
	cmd.Flags().String(prefix+"-path", "", what+" object key or prefix")
	cmd.Flags().String(prefix+"-endpoint", "", what+" S3 endpoint URL (omit for AWS)")
	cmd.Flags().String(prefix+"-region", "", what+" S3 region")
	cmd.Flags().String(prefix+"-access-key", "", what+" S3 access key")
	cmd.Flags().String(prefix+"-secret-key", "", what+" S3 secret key")
}

// buildJob assembles a CreateJobRequest body from the flags.
func buildJob(cmd *cobra.Command) (map[string]interface{}, error) {
	input := map[string]interface{}{}
	inputURL, _ := cmd.Flags().GetString("input-url")
	inS3 := s3Flags(cmd, "input")
	switch {
	case inputURL != "" && len(inS3) > 0:
		return nil, fmt.Errorf("use either --input-url or the --input-* S3 flags, not both")
	case inputURL != "":
		input["source"] = "url"
		input["url"] = inputURL
	case len(inS3) > 0:
		if inS3["bucket"] == nil {
			return nil, fmt.Errorf("--input-bucket is required for an S3 input")
		}
		input["source"] = "s3"
		input["s3"] = inS3
	default:
		return nil, fmt.Errorf("an input is required: --input-url or --input-bucket")
	}

	outS3 := s3Flags(cmd, "output")
	if outS3["bucket"] == nil {
		return nil, fmt.Errorf("--output-bucket is required")
	}

	outputs := []interface{}{}
	formats, _ := cmd.Flags().GetStringSlice("format")
	for _, f := range formats {
		outputs = append(outputs, map[string]interface{}{"type": strings.TrimSpace(f)})
	}
	specs, _ := cmd.Flags().GetStringArray("spec")
	for _, raw := range specs {
		var spec map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return nil, fmt.Errorf("invalid --spec %q: %w", raw, err)
		}
		outputs = append(outputs, spec)
	}
	if len(outputs) == 0 {
		return nil, fmt.Errorf("at least one --format or --spec is required")
	}

	body := map[string]interface{}{
		"input":   input,
		"output":  map[string]interface{}{"s3": outS3},
		"outputs": outputs,
	}
	if v, _ := cmd.Flags().GetString("webhook-url"); v != "" {
		body["webhook_url"] = v
	}
	if v, _ := cmd.Flags().GetString("idempotency-key"); v != "" {
		body["idempotency_key"] = v
	}
	return body, nil
}

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Submit a transcoding job",
		Long: `Submit a transcoding job. Give the source with --input-url or the --input-*
S3 flags, the destination with the --output-* flags, and one or more outputs
with --format (file, hls, thumbnails, gif) or --spec (a JSON output spec with
per-format settings). --file sends a complete JSON request instead.`,
		Example: `  cubecli transcoder create --input-url https://example.com/in.mp4 \
    --output-bucket videos --output-path out/ --output-endpoint https://eu.cubestorage.io \
    --output-region eu --output-access-key KEY --output-secret-key SECRET --format hls
  cubecli transcoder create --file job.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			var body interface{}
			if file, _ := cmd.Flags().GetString("file"); file != "" {
				var b map[string]interface{}
				if err := cmdutil.ReadJSONFile(file, &b); err != nil {
					return err
				}
				body = b
			} else {
				b, err := buildJob(cmd)
				if err != nil {
					return err
				}
				body = b
			}

			s := output.NewSpinner("Submitting job...")
			s.Start()
			resp, err := client.Post("/transcoder/jobs", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			if err := renderJob("Transcoding Job Submitted", resp); err != nil {
				return err
			}
			output.PrintSuccess("Job submitted")
			return nil
		},
	}
	cmd.Flags().String("file", "", "JSON request body (\"-\" for stdin); replaces the other flags")
	cmd.Flags().String("input-url", "", "Source video URL")
	addS3Flags(cmd, "input", "Source")
	addS3Flags(cmd, "output", "Destination")
	cmd.Flags().StringSlice("format", nil, "Output type: file, hls, thumbnails or gif (repeatable)")
	cmd.Flags().StringArray("spec", nil, `Output spec as JSON, e.g. '{"type":"file","codec":"h264"}' (repeatable)`)
	cmd.Flags().String("webhook-url", "", "URL notified when the job finishes")
	cmd.Flags().String("idempotency-key", "", "Retry-safe key: resubmitting with it returns the same job")
	return cmd
}

// --- batch ---

func batchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Submit up to 1000 jobs sharing one destination and output spec",
		Long: `Submit a batch of jobs from a JSON file with the fields output, outputs,
inputs (1-1000 items of {url} or {s3} or {path, out_subpath}), and optionally
input_defaults and webhook_url.`,
		Example: `  cubecli transcoder batch --file batch.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			file, _ := cmd.Flags().GetString("file")

			var body map[string]interface{}
			if err := cmdutil.ReadJSONFile(file, &body); err != nil {
				return err
			}

			s := output.NewSpinner("Submitting batch...")
			s.Start()
			resp, err := client.Post("/transcoder/jobs/batch", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				BatchID string   `json:"batch_id"`
				JobIDs  []string `json:"job_ids"`
				Count   int      `json:"count"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			output.PrintSuccess(fmt.Sprintf("Batch %s submitted with %d jobs", r.BatchID, r.Count))
			output.PrintInfo("Follow it with: cubecli transcoder list --batch " + r.BatchID)
			return nil
		},
	}
	cmd.Flags().String("file", "", "JSON request body (\"-\" for stdin)")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

// --- outputs ---

func outputsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "outputs <job_uuid>",
		Short: "List the files a finished job wrote to your bucket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching outputs...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/transcoder/jobs/%s/outputs", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				Outputs []struct {
					Type   string `json:"type"`
					Bucket string `json:"bucket"`
					Key    string `json:"key"`
				} `json:"outputs"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Job Outputs", []string{"Type", "Bucket", "Key"})
			for _, o := range r.Outputs {
				t.AddRow(o.Type, o.Bucket, o.Key)
			}
			t.Render()
			return nil
		},
	}
}

// --- cancel ---

func cancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <job_uuid>",
		Short: "Cancel a job that has not finished",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Cancel job %s?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Canceling job...")
			s.Start()
			resp, err := client.Delete("/transcoder/jobs/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Job canceled")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}
