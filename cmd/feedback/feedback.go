// Package feedback implements `cubecli feedback`.
package feedback

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

const maxMessageRunes = 5000

// maxStdinBytes bounds what is read from a pipe: 5000 runes are at most 20000 bytes.
const maxStdinBytes = 64 * 1024

var topics = []string{
	"vps", "baremetal", "kubernetes", "networking", "loadbalancers", "storage", "cdn",
	"dns_domains", "billing", "account", "dashboard", "support", "other",
}

var sentiments = []string{"very_negative", "negative", "positive", "very_positive"}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback [message...]",
		Short: "Send feedback to CubePath",
		Long: `Tell CubePath what you love or what we should improve. Your feedback is read by the
CubePath team.

The message is the words after the command. Without words, it is read from standard
input: piped or redirected text is used as is, and in a terminal you are prompted
for it. A message has 1 to 5000 characters.

--topic and --sentiment are optional and help the team route and weigh your message.
  topics:     ` + strings.Join(topics, ", ") + `
  sentiments: ` + strings.Join(sentiments, ", ") + `

The server accepts 3 messages per hour from the same IP address.`,
		Example: `  cubecli feedback "Creating a VPS from the CLI is great"
  cubecli feedback --topic billing --sentiment negative the invoice page is confusing
  echo "Please add a dark theme" | cubecli feedback --topic dashboard
  cubecli feedback --topic support < notes.txt`,
		RunE: run,
	}

	cmd.Flags().String("topic", "", "Area the feedback is about ("+strings.Join(topics, "|")+")")
	cmd.Flags().String("sentiment", "", "How you feel about it ("+strings.Join(sentiments, "|")+")")
	_ = cmd.RegisterFlagCompletionFunc("topic", completeFrom(topics))
	_ = cmd.RegisterFlagCompletionFunc("sentiment", completeFrom(sentiments))
	return cmd
}

func completeFrom(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

func run(cmd *cobra.Command, args []string) error {
	topic, _ := cmd.Flags().GetString("topic")
	sentiment, _ := cmd.Flags().GetString("sentiment")
	if topic != "" && !contains(topics, topic) {
		return fmt.Errorf("invalid --topic %q: use one of %s", topic, strings.Join(topics, ", "))
	}
	if sentiment != "" && !contains(sentiments, sentiment) {
		return fmt.Errorf("invalid --sentiment %q: use one of %s", sentiment, strings.Join(sentiments, ", "))
	}

	var raw string
	if len(args) > 0 {
		raw = strings.Join(args, " ")
	} else {
		var err error
		if raw, err = readMessage(cmd); err != nil {
			return err
		}
	}
	message, err := validateMessage(raw)
	if err != nil {
		return err
	}

	body := map[string]string{"message": message}
	if topic != "" {
		body["topic"] = topic
	}
	if sentiment != "" {
		body["sentiment"] = sentiment
	}

	client := cmdutil.GetClient(cmd)
	s := output.NewSpinner("Sending feedback...")
	s.Start()
	resp, err := client.Post("/feedback", body)
	s.Stop()
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
			msg := apiErr.Error()
			if apiErr.RetryAfter != "" {
				msg += fmt.Sprintf(" (try again in %s seconds)", apiErr.RetryAfter)
			}
			return errors.New(msg)
		}
		return err
	}

	if cmdutil.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(resp))
	}

	var result struct {
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(resp, &result)
	if result.Detail == "" {
		result.Detail = "Thank you for your feedback"
	}
	output.PrintSuccess(result.Detail)
	return nil
}

// readMessage takes the message from stdin: piped text as is, a prompt in a terminal.
func readMessage(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && f == os.Stdin && cmdutil.StdinIsTerminal() {
		fmt.Fprint(os.Stderr, "Your feedback (Enter to send): ")
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read feedback: %w", err)
		}
		return line, nil
	}
	data, err := io.ReadAll(io.LimitReader(in, maxStdinBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read feedback from stdin: %w", err)
	}
	return string(data), nil
}

// validateMessage mirrors the server: trimmed, 1..5000 code points, no control
// characters except tab and newline. CRLF is normalised to LF first.
func validateMessage(raw string) (string, error) {
	message := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n"))
	if message == "" {
		return "", errors.New("the feedback message is empty: pass it as arguments, pipe it in or type it at the prompt")
	}
	if n := utf8.RuneCountInString(message); n > maxMessageRunes {
		return "", fmt.Errorf("the feedback message is too long: %d characters, the maximum is %d", n, maxMessageRunes)
	}
	for _, r := range message {
		if r != '\t' && r != '\n' && unicode.IsControl(r) {
			return "", fmt.Errorf("the feedback message contains a control character (U+%04X)", r)
		}
	}
	return message, nil
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
