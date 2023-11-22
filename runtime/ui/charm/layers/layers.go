package layers

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/dive/dive/image"
)

type Model struct {
	index  int
	layers table.Model
}

var (
	ListStyle = lipgloss.NewStyle().
			Width(35).
			MarginTop(1).
			PaddingRight(3).
			MarginRight(3).
			Border(lipgloss.RoundedBorder())
	ListColorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3d719c"))
	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(4)
	ListSelectedListItemStyle = lipgloss.NewStyle().
					PaddingLeft(2).
					Foreground(lipgloss.Color("#569cd6"))
	DetailStyle = lipgloss.NewStyle().
			PaddingTop(2)
	DividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}).
			PaddingTop(1).
			PaddingBottom(1)
	TableMainStyle = lipgloss.NewStyle().
			Align(lipgloss.Center)
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#569cd6")).
				Bold(true)
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569cd6")).
			PaddingBottom(1).
			Bold(true).
			Underline(true).
			Inline(true)
)

func (Model) Init() tea.Cmd {
	return nil
}

// returns the total number of rows to show
func (m Model) Height() int {
	return m.layers.Height()
}

func (m Model) Cursor() uint {
	return uint(m.layers.Cursor())
}

// pass the available number of rows for display
func (m *Model) SetHeight(h int) {
	log.Printf("Setting layers height from %d to %d", m.layers.Height(), h)
	m.layers.SetHeight(h)
	log.Printf("Set to %d", m.layers.Height())
}

// pass the available number of rows for display
func (m *Model) SetWidth(w int) {
	m.layers.SetWidth(w)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd = nil
	m.layers, cmd = m.layers.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	log.Printf("returning layers view, total %d rows, height: %d", strings.Count(m.layers.View(), "\n"), m.layers.Width())
	return m.layers.View()
}

func New(layers []*image.Layer) Model {
	rows := []table.Row{}
	for _, l := range layers {
		rows = append(rows, table.Row{fmt.Sprint(l.Size), l.Command})
	}
	log.Printf("Number of layers: %d", len(layers))
	columns := []table.Column{
		{Title: "Size", Width: 10},
		{Title: "Command", Width: 100}, // TODO: how to tell it to use all the available space
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		// table.WithHeight(200),
	)
	return Model{
		layers: t,
	}
}
