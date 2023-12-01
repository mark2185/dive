package tui

import (
	// "strings"

	"log"
	_ "os"
	_ "strings"

	"github.com/76creates/stickers/flexbox"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
	uifiletree "github.com/wagoodman/dive/runtime/ui/charm/filetree"
	"github.com/wagoodman/dive/runtime/ui/charm/layer_details"
	"github.com/wagoodman/dive/runtime/ui/charm/layers"
	_ "golang.org/x/term"
)

// keyMap defines a set of keybindings. To work for help it must satisfy
// key.Map. It could also very easily be a map[string]key.Binding.
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Help   key.Binding
	Switch key.Binding
	Quit   key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Switch: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("<Tab>", "Switch view"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right}, // first column
		{k.Help, k.Quit},                // second column
	}
}

type styles struct {
	BorderColor lipgloss.Color
	Table       lipgloss.Style
}

func DefaultStyles() styles {
	var s styles
	s.BorderColor = lipgloss.Color("36")
	s.Table = lipgloss.
		NewStyle().
		BorderForeground(s.BorderColor).
		BorderStyle(lipgloss.NormalBorder()).
		Padding(1).
		Width(80)
	return s
}

type SelectedBox int

const (
	Layer SelectedBox = iota
	LayerDetails
	Filetree
)

type model struct {
	selected     SelectedBox
	box          *flexbox.FlexBox
	layers       layers.Model
	layerDetails layer_details.Model
	filetree     uifiletree.Model
	width        int
	height       int
	help         help.Model
	keys         keyMap
	viewport     viewport.Model
	styles       styles
}

var (
	style1 = lipgloss.NewStyle().Background(lipgloss.Color("#fc5c65"))
	style2 = lipgloss.NewStyle().Background(lipgloss.Color("#fd9644"))
	style3 = lipgloss.NewStyle().Background(lipgloss.Color("#fed330"))
	style4 = lipgloss.NewStyle().Background(lipgloss.Color("#26de81"))
)

func New(analysis *image.AnalysisResult, treeStack filetree.Comparer) model {
	ls := analysis.Layers
	trees := analysis.RefTrees
	m := model{
		selected:     Layer,
		layers:       layers.New(ls),
		layerDetails: layer_details.New(ls),
		filetree:     uifiletree.New(trees),
		help:         help.New(),
		keys:         keys,
		styles:       DefaultStyles(),
		box:          flexbox.New(0, 0),
	}
	rows := []*flexbox.Row{
		m.box.NewRow().AddCells(
			// layers
			flexbox.NewCell(1, 50),
			// layer details
			flexbox.NewCell(1, 50),
		),
		m.box.NewRow().AddCells(
			// filetree
			flexbox.NewCell(1, 50),
		),
		m.box.NewRow().AddCells(
			// help
			flexbox.NewCell(1, 2),
		),
	}
	m.box.AddRows(rows)
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Println("Update method called")
	var cmd tea.Cmd = nil

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.help.Width = msg.Width

		log.Printf("Received resize: wxh: %dx%d", m.width, m.height)
		m.box.SetWidth(msg.Width)
		m.box.SetHeight(msg.Height)

		m.box.ForceRecalculate()

		_r := m.box.GetRow(0)
		_c := _r.GetCell(0)
		log.Printf("Cell wxh, %dx%d", _c.GetWidth(), _c.GetHeight())
		m.layers.SetHeight(_c.GetHeight() - 1)
		m.layers.SetWidth(_c.GetWidth())

		m.layerDetails.Resize(_c.GetWidth(), _c.GetHeight())

		_r1 := m.box.GetRow(1)
		_c1 := _r1.GetCell(0)
		m.filetree.SetHeight(_c1.GetHeight() - 1)

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Help) {
			m.help.ShowAll = !m.help.ShowAll
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up, m.keys.Down):
			switch m.selected {
			case Layer:
				m.layers, cmd = m.layers.Update(msg)
				m.filetree.SetIndex(m.layers.Cursor())
				m.layerDetails.Index = m.layers.Cursor()
			case Filetree:
				m.filetree, cmd = m.filetree.Update(msg)
			}
		case key.Matches(msg, m.keys.Switch):
			switch m.selected {
			case Layer:
				m.layers.Blur()
				m.filetree.Focus()
				m.layers, cmd = m.layers.Update(msg)
				m.filetree, cmd = m.filetree.Update(msg)
				m.selected = Filetree
			case Filetree:
				m.filetree.Blur()
				m.layers.Focus()
				m.layers, cmd = m.layers.Update(msg)
				m.filetree, cmd = m.filetree.Update(msg)
				m.selected = Layer
			}
		}
	}

	return m, cmd
}

func (m model) View() string {
	if m.height <= 20 {
		return "Screen too small, please increase to at least 20 rows"
	}
	log.Printf("Layers' height is: %d", m.layers.Height())
	log.Printf("H: %d, W: %d", m.height, m.width)
	m.box.GetRow(0).GetCell(0).SetContent(m.layers.View())
	m.box.GetRow(0).GetCell(1).SetContent(m.layerDetails.View())
	m.box.GetRow(1).GetCell(0).SetContent(m.filetree.View())
	m.box.GetRow(2).GetCell(0).SetContent(m.help.View(m.keys))
	return m.box.Render()
	// helpView := m.help.View(m.keys)
	// if m.help.ShowAll {
	// // TODO: raise popup with keybindings
	// }

	// helpHeight := lipgloss.Height(helpView)
	// layersHeight := lipgloss.Height(m.layers.View())

	// availableHeight := m.height - helpHeight

	// log.Printf("Total height: %d, help height: %d, layersHeight: %d", totalHeight, helpHeight, layersHeight)
	// log.Printf("Table height: %d", m.layers.Height())

	// numberOfPaddingLines :=
	// log.Printf("Number of padding lines: %d", numberOfPaddingLines)
	// bubbleStyle := lipgloss.NewStyle().
	// PaddingLeft(1).
	// PaddingRight(1).
	// BorderStyle(lipgloss.NormalBorder())

	// //paddingString := strings.Repeat("\n", numberOfPaddingLines)
	// return bubbleStyle.Render(lipgloss.JoinVertical(lipgloss.Top, m.layers.View(), helpView))
	//return "\n" + m.layers.View() + strings.Repeat("\n", totalHeight-layersHeight-helpHeight) + helpView
	// return lipgloss.Place(
	// m.width,
	// m.height,
	// lipgloss.Top,
	// lipgloss.Top,
	// lipgloss.JoinVertical(
	// lipgloss.Top,
	// m.layers.View(),
	// helpView,
	// ),
	// )
	// return "\n" + m.layers.View() + "\n" + helpView
}
