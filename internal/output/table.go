package output

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

type Table struct {
	title   string
	headers []string
	rows    [][]string
}

func NewTable(title string, headers []string) *Table {
	return &Table{
		title:   title,
		headers: headers,
	}
}

func (t *Table) AddRow(values ...string) {
	t.rows = append(t.rows, values)
}

func (t *Table) Render() {
	if len(t.rows) == 0 {
		fmt.Printf("No %s found.\n", t.title)
		return
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Padding(0, 1)

	cellStyle := lipgloss.NewStyle().
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	rows := make([][]string, len(t.rows))
	copy(rows, t.rows)

	tbl := table.New().
		Headers(t.headers...).
		Rows(rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	fmt.Fprintf(os.Stdout, "\n%s\n", t.title)
	fmt.Fprintln(os.Stdout, tbl)
}
