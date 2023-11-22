package layer_details

import (
	"fmt"
	_ "fmt"
	_ "log"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/dive/dive/image"
)

type Model struct {
	layerDetails []string
	index        uint
	viewport     viewport.Model
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

func (m *Model) Resize(width int, height int) {
	m.viewport = viewport.New(width, height)
}

func (m *Model) SetCursor(index uint) {
	m.index = index
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd = nil
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	m.viewport.SetContent(m.layerDetails[m.index])
	return m.viewport.View()
}

func New(layers []*image.Layer) Model {
	details := []string{}
	for _, l := range layers {
		details = append(details, fmt.Sprintf("ID: %s\nDigest: %s\nCommand: \n%s\n", l.ShortId(), l.Digest, l.Command))
	}
	return Model{
		layerDetails: details,
		index:        0,
		viewport:     viewport.New(0, 0),
	}
}
