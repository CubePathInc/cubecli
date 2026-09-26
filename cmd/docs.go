package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// `cubecli docs markdown <dir>` writes a compact command reference, one file per
// command group. It feeds the CubePath agent skills, whose CI regenerates it and
// fails when it drifts, so the skills never document flags that do not exist.
func init() {
	docsCmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate documentation",
		Hidden: true,
	}
	docsCmd.AddCommand(&cobra.Command{
		Use:   "markdown <dir>",
		Short: "Write a Markdown command reference into dir",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeMarkdownReference(rootCmd, args[0])
		},
	})
	rootCmd.AddCommand(docsCmd)
}

func writeMarkdownReference(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var index strings.Builder
	index.WriteString("# cubecli command reference\n\n")
	index.WriteString("Generated from the cubecli command tree with `cubecli docs markdown`. Do not edit by hand.\n\n")
	index.WriteString("## Global flags\n\n")
	writeFlags(&index, root.PersistentFlags())
	index.WriteString("\n## Command groups\n\n")

	for _, group := range visibleChildren(root) {
		if group.Name() == "docs" {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# cubecli %s\n\n%s\n", group.Name(), group.Short)
		writeCommandTree(&b, group)
		file := group.Name() + ".md"
		if err := os.WriteFile(filepath.Join(dir, file), []byte(b.String()), 0644); err != nil {
			return err
		}
		fmt.Fprintf(&index, "- [%s](%s): %s\n", group.Name(), file, group.Short)
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(index.String()), 0644)
}

func writeCommandTree(b *strings.Builder, c *cobra.Command) {
	if c.Runnable() {
		fmt.Fprintf(b, "\n## `%s`\n\n", c.CommandPath())
		if c.Short != "" {
			fmt.Fprintf(b, "%s\n\n", c.Short)
		}
		if c.Long != "" && c.Long != c.Short {
			fmt.Fprintf(b, "%s\n\n", strings.TrimSpace(c.Long))
		}
		fmt.Fprintf(b, "Usage: `%s`\n", c.UseLine())
		if len(c.Aliases) > 0 {
			fmt.Fprintf(b, "\nAliases: %s\n", strings.Join(c.Aliases, ", "))
		}
		if c.HasAvailableLocalFlags() {
			b.WriteString("\n")
			writeFlags(b, c.LocalNonPersistentFlags())
		}
		if c.Example != "" {
			fmt.Fprintf(b, "\nExamples:\n\n```\n%s\n```\n", strings.TrimRight(c.Example, "\n"))
		}
	}
	for _, sub := range visibleChildren(c) {
		writeCommandTree(b, sub)
	}
}

func writeFlags(b *strings.Builder, flags *pflag.FlagSet) {
	var lines []string
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		if t := f.Value.Type(); t != "bool" {
			name += " " + t
		}
		line := fmt.Sprintf("- `%s`: %s", name, f.Usage)
		if _, required := f.Annotations[cobra.BashCompOneRequiredFlag]; required {
			line += " (required)"
		} else if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "[]" && f.DefValue != "0" {
			line += fmt.Sprintf(" (default %s)", f.DefValue)
		}
		lines = append(lines, line)
	})
	sort.Strings(lines)
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
}

func visibleChildren(c *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, sub := range c.Commands() {
		if sub.Hidden || !sub.IsAvailableCommand() || sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		out = append(out, sub)
	}
	return out
}
