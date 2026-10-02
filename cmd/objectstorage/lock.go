package objectstorage

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// lockRetention is a default retention rule: a mode and exactly one of days or years.
type lockRetention struct {
	Mode  string `json:"mode"`
	Days  *int   `json:"days"`
	Years *int   `json:"years"`
}

// objectLock is the "object_lock" object of buckets.
type objectLock struct {
	Enabled          bool           `json:"enabled"`
	DefaultRetention *lockRetention `json:"default_retention"`
}

const complianceWarning = "Compliance retention cannot be shortened or removed by anyone, CubePath included, " +
	"and the protected versions keep being billed until their retention ends."

// formatRetention renders a rule as "governance 30d" or "compliance 1y".
func formatRetention(r *lockRetention) string {
	if r == nil {
		return "none"
	}
	switch {
	case r.Days != nil:
		return fmt.Sprintf("%s %dd", r.Mode, *r.Days)
	case r.Years != nil:
		return fmt.Sprintf("%s %dy", r.Mode, *r.Years)
	}
	return r.Mode
}

// formatLockColumn is the short "Lock" column of bucket list.
func formatLockColumn(l objectLock) string {
	if !l.Enabled {
		return "-"
	}
	if l.DefaultRetention == nil {
		return "on"
	}
	return formatRetention(l.DefaultRetention)
}

// formatLock is the "Object Lock" row of bucket get.
// bucketEncryption is the bucket's encryption at rest (SSE-S3); the API sends
// null while it is off.
type bucketEncryption struct {
	Algorithm string `json:"algorithm"`
	Scope     string `json:"scope"`
	AppliedAt string `json:"applied_at"`
}

func formatEncryption(e *bucketEncryption) string {
	if e == nil {
		return "off"
	}
	if e.Scope == "new_objects" {
		return e.Algorithm + " (new objects; older ones are being encrypted)"
	}
	return e.Algorithm + " (all objects)"
}

func formatLock(l objectLock) string {
	if !l.Enabled {
		return "off"
	}
	if l.DefaultRetention == nil {
		return "on, no default retention"
	}
	return "on, default retention " + formatRetention(l.DefaultRetention)
}

// parseRetention validates --mode/--days/--years (or their --lock-* names on
// create) and builds the API rule; it returns nil when no flag was given.
func parseRetention(mode string, days, years int, daysSet, yearsSet bool, flagPrefix string) (map[string]interface{}, error) {
	if mode == "" && !daysSet && !yearsSet {
		return nil, nil
	}
	if mode != "governance" && mode != "compliance" {
		return nil, fmt.Errorf("--%smode must be governance or compliance", flagPrefix)
	}
	if daysSet == yearsSet {
		return nil, fmt.Errorf("set either --%sdays or --%syears for the default retention", flagPrefix, flagPrefix)
	}
	rule := map[string]interface{}{"mode": mode}
	if daysSet {
		if days <= 0 {
			return nil, fmt.Errorf("--%sdays must be a positive whole number", flagPrefix)
		}
		rule["days"] = days
	} else {
		if years <= 0 {
			return nil, fmt.Errorf("--%syears must be a positive whole number", flagPrefix)
		}
		rule["years"] = years
	}
	return rule, nil
}

// confirmCompliance asks before a compliance rule unless --yes was given. Without
// a terminal it refuses instead of prompting.
func confirmCompliance(cmd *cobra.Command, msg string) (bool, error) {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return true, nil
	}
	if !cmdutil.StdinIsTerminal() {
		return false, fmt.Errorf("compliance retention needs confirmation: pass --yes")
	}
	output.PrintWarning(complianceWarning)
	return cmdutil.ConfirmAction(msg), nil
}

func bucketObjectLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "object-lock",
		Aliases: []string{"lock"},
		Short:   "Manage the default retention of a bucket with Object Lock",
		Long: `Object Lock keeps object versions from being deleted or overwritten until
their retention date (WORM). It can only be turned on when the bucket is created
(bucket create --object-lock); these commands change the bucket's default
retention, which applies to every version without a retention of its own.

  governance  keys created with --bypass-governance can still delete
  compliance  nobody can delete or shorten it before its date, CubePath included`,
	}
	cmd.AddCommand(bucketObjectLockSetCmd())
	return cmd
}

func bucketObjectLockSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <bucket>",
		Short: "Set or remove the default retention of a bucket with Object Lock",
		Long: `Set or remove the default retention of a bucket created with Object Lock.

A governance rule can be changed or removed at any time. A compliance rule can
only be kept or lengthened: it cannot be removed, shortened or turned into
governance. Turning compliance on or lengthening the retention requires
--accept-object-lock-terms.`,
		Example: `  cubecli s3 bucket object-lock set backups --mode governance --days 30 --accept-object-lock-terms
  cubecli s3 bucket object-lock set archive --mode compliance --years 7 --accept-object-lock-terms --yes
  cubecli s3 bucket object-lock set backups --remove`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			remove, _ := cmd.Flags().GetBool("remove")
			mode, _ := cmd.Flags().GetString("mode")
			days, _ := cmd.Flags().GetInt("days")
			years, _ := cmd.Flags().GetInt("years")
			accept, _ := cmd.Flags().GetBool("accept-object-lock-terms")

			var rule map[string]interface{}
			if remove {
				if mode != "" || cmd.Flags().Changed("days") || cmd.Flags().Changed("years") {
					return fmt.Errorf("--remove cannot be combined with --mode, --days or --years")
				}
			} else {
				var err error
				rule, err = parseRetention(mode, days, years, cmd.Flags().Changed("days"), cmd.Flags().Changed("years"), "")
				if err != nil {
					return err
				}
				if rule == nil {
					return fmt.Errorf("give --mode with --days or --years, or --remove")
				}
				if mode == "compliance" {
					ok, err := confirmCompliance(cmd, fmt.Sprintf("Set a compliance default retention on bucket %s?", args[0]))
					if err != nil {
						return err
					}
					if !ok {
						output.PrintWarning("Aborted")
						return nil
					}
				}
			}

			body := map[string]interface{}{
				"default_retention":        rule,
				"accept_object_lock_terms": accept,
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Updating default retention...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Put("/object-storage/buckets/"+uuid+"/object-lock", body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			if remove {
				output.PrintSuccess("Default retention removed")
			} else {
				output.PrintSuccess("Default retention updated")
			}
			return nil
		},
	}
	cmd.Flags().String("mode", "", "governance or compliance")
	cmd.Flags().Int("days", 0, "Retention in days")
	cmd.Flags().Int("years", 0, "Retention in years")
	cmd.Flags().Bool("remove", false, "Remove the default retention (not possible for compliance)")
	cmd.Flags().Bool("accept-object-lock-terms", false, "Accept the Object Lock terms (needed to turn compliance on or lengthen the retention)")
	cmd.Flags().BoolP("yes", "y", false, "Skip the compliance confirmation prompt")
	cmd.MarkFlagsMutuallyExclusive("days", "years")
	return cmd
}
