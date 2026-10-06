// Package org holds the commands about the organization itself.
package org

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// PlanPage is the dashboard page where the plan is bought, changed, cancelled
// or resumed. cubecli only reads the plan.
const PlanPage = "https://my.cubepath.com/organization/plan"

// NewCmd returns the `org` command group.
func NewCmd() *cobra.Command {
	orgCmd := &cobra.Command{
		Use:     "org",
		Aliases: []string{"organization"},
		Short:   "Inspect your organization",
	}
	orgCmd.AddCommand(newPlanCmd())
	return orgCmd
}

// USD is an amount the API sends either as a JSON number or as a decimal string.
type USD struct {
	Value float64
	Set   bool
}

func (u *USD) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*u = USD{}
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid amount %q", s)
	}
	*u = USD{Value: v, Set: true}
	return nil
}

// String renders the amount as $49.00, or "-" when absent.
func (u USD) String() string {
	if !u.Set {
		return "-"
	}
	return fmt.Sprintf("$%.2f", u.Value)
}

type planRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type planResponse struct {
	Subscription *struct {
		UUID                string   `json:"uuid"`
		Status              string   `json:"status"`
		Plan                planRef  `json:"plan"`
		BillingInterval     string   `json:"billing_interval"`
		AmountUSD           USD      `json:"amount_usd"`
		CurrentPeriodStart  *string  `json:"current_period_start"`
		CurrentPeriodEnd    *string  `json:"current_period_end"`
		PastDueSince        *string  `json:"past_due_since"`
		CancelAtPeriodEnd   bool     `json:"cancel_at_period_end"`
		NextPlan            *planRef `json:"next_plan"`
		NextBillingInterval *string  `json:"next_billing_interval"`
		Channel             string   `json:"channel"`
		Source              string   `json:"source"`
	} `json:"subscription"`
	Invoices []struct {
		InvoiceNumber string  `json:"invoice_number"`
		Status        string  `json:"status"`
		Amount        USD     `json:"amount"`
		Currency      string  `json:"currency"`
		DueDate       *string `json:"due_date"`
		Kind          string  `json:"kind"`
		PeriodStart   *string `json:"period_start"`
		PeriodEnd     *string `json:"period_end"`
	} `json:"invoices"`
	Credit *struct {
		MonthlyCreditUSD    USD `json:"monthly_credit_usd"`
		ThisMonthGrantedUSD USD `json:"this_month_granted_usd"`
	} `json:"credit"`
}

var perInterval = map[string]string{
	"monthly":      "per month",
	"quarterly":    "per quarter",
	"semiannually": "every 6 months",
	"yearly":       "per year",
}

// Day shortens an ISO datetime to its date ("2026-11-01T00:00:00" -> "2026-11-01").
func Day(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	if len(*s) > 10 && ((*s)[10] == 'T' || (*s)[10] == ' ') {
		return (*s)[:10]
	}
	return *s
}

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Show the organization's plan, renewal and scheduled changes",
		Long: `Show the commercial plan of the organization (Free, Pro, Business or
Enterprise): status, billing interval, price in USD, current period, renewal,
any change or cancellation scheduled for the end of the period, the monthly
plan credit and the plan invoices.

This command only reads the plan. Buy, change, cancel or resume it from the
Plan page of the dashboard: ` + PlanPage,
		Example: `  cubecli org plan
  cubecli org plan --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching plan...")
			s.Start()
			resp, err := client.Get("/organization/plan")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var p planResponse
			if err := json.Unmarshal(resp, &p); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			renderPlan(&p)
			return nil
		},
	}
}

func renderPlan(p *planResponse) {
	sub := p.Subscription
	if sub == nil || sub.Status == "lapsed" || sub.Status == "cancelled" {
		info := output.NewTable("Organization Plan", []string{"Field", "Value"})
		info.AddRow("Plan", "Free")
		if sub != nil {
			info.AddRow("Last plan", fmt.Sprintf("%s (%s)", sub.Plan.Name, output.FormatStatus(sub.Status)))
		}
		info.Render()
		renderInvoices(p)
		output.PrintInfo("Upgrade from the dashboard: " + PlanPage)
		return
	}

	info := output.NewTable("Organization Plan", []string{"Field", "Value"})
	info.AddRow("Plan", fmt.Sprintf("%s (%s)", sub.Plan.Name, sub.Plan.Code))
	info.AddRow("Status", output.FormatStatus(sub.Status))
	info.AddRow("Billing interval", sub.BillingInterval)
	price := sub.AmountUSD.String()
	if per, ok := perInterval[sub.BillingInterval]; ok && sub.AmountUSD.Set {
		price += " " + per
	}
	info.AddRow("Price (USD)", price)
	info.AddRow("Current period", fmt.Sprintf("%s to %s", Day(sub.CurrentPeriodStart), Day(sub.CurrentPeriodEnd)))
	switch {
	case sub.Status == "pending_payment":
		info.AddRow("Renewal", "starts once the first invoice is paid")
	case sub.CancelAtPeriodEnd:
		info.AddRow("Ends", Day(sub.CurrentPeriodEnd)+", does not renew")
	default:
		info.AddRow("Renews", Day(sub.CurrentPeriodEnd))
	}
	if sub.PastDueSince != nil {
		info.AddRow("Past due since", Day(sub.PastDueSince))
	}
	if change := scheduledChange(sub.Plan, sub.BillingInterval, sub.NextPlan, sub.NextBillingInterval); change != "" {
		info.AddRow("Scheduled change", change+" on "+Day(sub.CurrentPeriodEnd))
	}
	if p.Credit != nil && p.Credit.MonthlyCreditUSD.Set && p.Credit.MonthlyCreditUSD.Value > 0 {
		info.AddRow("Monthly credit", fmt.Sprintf("%s (granted this month: %s)", p.Credit.MonthlyCreditUSD, p.Credit.ThisMonthGrantedUSD))
	}
	if sub.Channel != "" {
		info.AddRow("Channel", sub.Channel)
	}
	info.Render()
	renderInvoices(p)

	switch sub.Status {
	case "past_due":
		output.PrintWarning("A plan invoice is overdue: pay it or the organization returns to Free. " + PlanPage)
	case "pending_payment":
		output.PrintWarning("The plan starts once its invoice is paid: " + PlanPage)
	}
}

// scheduledChange describes the plan and/or interval that apply at the end of
// the period, or "" when nothing is scheduled.
func scheduledChange(cur planRef, curInterval string, next *planRef, nextInterval *string) string {
	planChanges := next != nil && next.Code != "" && next.Code != cur.Code
	intervalChanges := nextInterval != nil && *nextInterval != "" && *nextInterval != curInterval
	if !planChanges && !intervalChanges {
		return ""
	}
	name, interval := cur.Name, curInterval
	if planChanges {
		name = next.Name
	}
	if intervalChanges {
		interval = *nextInterval
	}
	return fmt.Sprintf("to %s (%s)", name, interval)
}

func renderInvoices(p *planResponse) {
	if len(p.Invoices) == 0 {
		return
	}
	t := output.NewTable("Plan Invoices", []string{"Invoice", "Kind", "Status", "Amount", "Due", "Period"})
	for _, inv := range p.Invoices {
		amount := inv.Amount.String()
		if inv.Currency != "" && inv.Currency != "USD" && inv.Amount.Set {
			amount = fmt.Sprintf("%.2f %s", inv.Amount.Value, inv.Currency)
		}
		t.AddRow(inv.InvoiceNumber, inv.Kind, output.FormatStatus(inv.Status), amount, Day(inv.DueDate),
			fmt.Sprintf("%s to %s", Day(inv.PeriodStart), Day(inv.PeriodEnd)))
	}
	t.Render()
}

// AccountPlan is the `plan` block of GET /account/me (null = Free).
type AccountPlan struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	Status            string  `json:"status"`
	RenewsAt          *string `json:"renews_at"`
	CancelAtPeriodEnd bool    `json:"cancel_at_period_end"`
}

// Summary renders the plan in one cell: "Free", "Pro, renews 2026-11-01",
// "Pro (past_due), renews 2026-11-01" or "Pro, ends 2026-11-01".
func (a *AccountPlan) Summary() string {
	if a == nil {
		return "Free"
	}
	s := a.Name
	if s == "" {
		s = a.Code
	}
	if a.Status != "" && a.Status != "active" {
		s += " (" + a.Status + ")"
	}
	if a.RenewsAt != nil && *a.RenewsAt != "" {
		if a.CancelAtPeriodEnd {
			s += ", ends " + Day(a.RenewsAt)
		} else {
			s += ", renews " + Day(a.RenewsAt)
		}
	}
	return s
}
