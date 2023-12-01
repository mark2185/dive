package filetree

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/dive/dive/filetree"
)

type Model struct {
	view       table.Model
	layerIndex int                  // which layer is currently selected
	layers     []*filetree.FileTree // each layer is a tree
	treeViews  map[int]tableView    // cache
}

type tableView struct {
	rows []table.Row
}

func (t *tableView) Add(row table.Row) {
	t.rows = append(t.rows, row)
}

func (Model) Init() tea.Cmd {
	return nil
}

func (m *Model) SetHeight(h int) {
	m.view.SetHeight(h)
}

func (m *Model) SetIndex(index int) {
	m.layerIndex = index
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd = nil
	// TODO: collapse on space
	// TODO: page up/down
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

func (m *Model) Focus() {
	m.view.Focus()
}

func (m *Model) Blur() {
	m.view.Blur()
}

func NewTreeView(f *filetree.FileTree) tableView {
	res := tableView{}
	for _, fnm := range f.Sort() {
		fn := fnm.Node

		res.Add(
			table.Row{
				fn.Metadata.FileInfo.Mode.Perm().String(),
				fmt.Sprint(fn.Metadata.FileInfo.Uid),
				fmt.Sprint(fn.Metadata.FileInfo.Gid),
				fmt.Sprint(fn.Size),
				strings.Repeat("-", fnm.Depth) + fn.Name,
			})
	}
	return res
}

func (m Model) View() string {
	// check if it exists in cache
	if _, ok := m.treeViews[m.layerIndex]; !ok {
		m.treeViews[m.layerIndex] = NewTreeView(m.layers[m.layerIndex])
	}
	m.view.SetRows(m.treeViews[m.layerIndex].rows)
	return m.view.View()
}

func New(trees []*filetree.FileTree) Model {
	columns := []table.Column{
		{Title: "Permission", Width: 10},
		{Title: "UID", Width: 4},
		{Title: "GID", Width: 4},
		{Title: "Size", Width: 10},
		{Title: "Filetree", Width: 120},
	}
	treeView := NewTreeView(trees[0])
	return Model{
		view: table.New(
			table.WithColumns(columns),
			table.WithRows(treeView.rows),
		),
		layers:    trees,
		treeViews: map[int]tableView{0: treeView},
	}
}
