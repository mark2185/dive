package filetree

import (
	"archive/tar"
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/phayes/permbits"
	"github.com/sirupsen/logrus"
)

const (
	AttributeFormat = "%s%s %11s %10s "
)

// TODO: this should be in the TUI package
var diffTypeColor = map[DiffType]*color.Color{
	Added:      color.New(color.FgGreen),
	Removed:    color.New(color.FgRed),
	Modified:   color.New(color.FgYellow),
	Unmodified: color.New(color.Reset),
}

// FileNode represents a single file, its relation to files beneath it, and its metadata.
type FileNode struct {
	Parent   *FileNode // needed to remove itself from the parent's children
	Size     int64     // memoized total size of file or directory
	Name     string    // basename of path
	Metadata NodeData
	Children map[string]*FileNode // keys are the names of nods
	path     string               // absolute path to node
}

func (node *FileNode) View() string {
	return ""
}

// NewNode creates a new FileNode relative to the given parent node with a payload.
func NewNode(parent *FileNode, name string, data FileInfo) *FileNode {
	return &FileNode{
		Name: name,
		Metadata: NodeData{
			FileInfo: data,
			DiffType: Unmodified,
		},
		Size:     -1,
		Children: make(map[string]*FileNode),
		Parent:   parent,
	}
}

// renderTreeLine returns a string representing this FileNode in the context of a greater ASCII tree.
// TODO: likely deprecated with charm
func (node *FileNode) renderTreeLine(spaces []bool, last bool, collapsed bool) string {
	var otherBranches string
	for _, space := range spaces {
		if space {
			otherBranches += noBranchSpace
		} else {
			otherBranches += branchSpace
		}
	}

	thisBranch := middleItem
	if last {
		thisBranch = lastItem
	}

	collapsedIndicator := uncollapsedItem
	if collapsed {
		collapsedIndicator = collapsedItem
	}

	return otherBranches + thisBranch + collapsedIndicator + node.String() + newLine
}

// Copy duplicates the existing node relative to a new parent node.
// TODO: copy should not take arguments, it should just return a copy of itself
func (node *FileNode) Copy(parent *FileNode) *FileNode {
	newNode := NewNode(parent, node.Name, node.Metadata.FileInfo)
	newNode.Metadata.ViewInfo = node.Metadata.ViewInfo
	newNode.Metadata.DiffType = node.Metadata.DiffType
	for name, child := range node.Children {
		newNode.Children[name] = child.Copy(newNode)
		child.Parent = newNode
	}
	return newNode
}

// AddChild creates a new node relative to the current FileNode.
func (node *FileNode) AddChild(name string, data FileInfo) (child *FileNode) {
	// never allow processing of purely whiteout flag files (for now)
	if strings.HasPrefix(name, doubleWhiteoutPrefix) {
		// TODO: treat as file with size 0?
		return nil
	}

	child = NewNode(node, name, data)
	if node.Children[name] != nil {
		// tree node already exists, replace the payload, keep the children
		// TODO: investigate what FileInfo contains
		node.Children[name].Metadata.FileInfo = *data.Copy()
	} else {
		node.Children[name] = child
		// TODO: move this to Tree
		// node.Tree.Count++
	}

	return child
}

// Remove deletes the current FileNode and all its children.
// Also removes itself from parent's children.
func (node *FileNode) Remove() error {
	// TODO: FileNode should not have concept of a tree it belongs to
	// if node == node.Tree.Root {
	// return fmt.Errorf("cannot remove the tree root")
	// }
	for _, child := range node.Children {
		err := child.Remove()
		if err != nil {
			return err
		}
	}
	delete(node.Parent.Children, node.Name)
	// TODO: FileNode should not have concept of a tree it belongs to
	// node.Tree.Size--
	return nil
}

// String shows the filename formatted into the proper color (by DiffType), additionally indicating if it is a symlink.
// TODO: investigate symlinks in .tars
func (node *FileNode) String() string {
	var display string
	if node == nil {
		return ""
	}

	display = node.Name
	if node.Metadata.FileInfo.TypeFlag == tar.TypeSymlink || node.Metadata.FileInfo.TypeFlag == tar.TypeLink {
		display += " → " + node.Metadata.FileInfo.Linkname
	}
	return diffTypeColor[node.Metadata.DiffType].Sprint(display)
}

// MetadataString returns the FileNode metadata in a columnar string.
// TODO: likely needed for the UID:GID column
func (node *FileNode) MetadataString() string {
	if node == nil {
		return ""
	}

	fileMode := permbits.FileMode(node.Metadata.FileInfo.Mode).String()
	dir := "-"
	if node.Metadata.FileInfo.IsDir {
		dir = "d"
	}
	user := node.Metadata.FileInfo.Uid
	group := node.Metadata.FileInfo.Gid
	userGroup := fmt.Sprintf("%d:%d", user, group)

	// don't include file sizes of children that have been removed (unless the node in question is a removed dir,
	// then show the accumulated size of removed files)
	sizeBytes := node.CalculateSize()

	size := humanize.Bytes(uint64(sizeBytes))

	return diffTypeColor[node.Metadata.DiffType].Sprint(fmt.Sprintf(AttributeFormat, dir, fileMode, userGroup, size))
}

func (node *FileNode) CalculateSize() int64 {
	// -1 is a placeholder size value
	if -1 != node.Size {
		return node.Size
	}
	if node.Metadata.DiffType == Removed {
		return 0
	}

	var currentNodeSize int64 = 0
	// TODO: I sure hope there aren't loops in the tree
	// TODO: rewrite without recursion if benchmarks show this part is slow
	for _, n := range node.Children {
		currentNodeSize += n.CalculateSize()
	}
	return node.Size
}

func (node *FileNode) GetSize() int64 {
	// node.Size == -1 is a sentinel value,
	// it forces the recalculation of the actual size which is then cached in the object
	if -1 != node.Size {
		return node.Size
	}

	var sizeBytes int64
	if node.IsLeaf() {
		// TODO: use the visitor pattern to simplify
		sizeBytes = node.Metadata.FileInfo.Size
	} else {
		sizer := func(curNode *FileNode) error {
			if curNode.Metadata.DiffType != Removed || node.Metadata.DiffType == Removed {
				sizeBytes += curNode.Metadata.FileInfo.Size
			}
			return nil
		}
		err := node.VisitDepthChildFirst(sizer, nil, nil)
		if err != nil {
			logrus.Errorf("unable to propagate node for metadata: %+v", err)
		}
	}
	node.Size = sizeBytes
	return node.Size
}

// VisitDepthChildFirst iterates a tree depth-first (starting at this FileNode),
// evaluating the deepest depths first (visit on bubble up)
// TODO: why doesn't this have an early exit?
// TODO: extract visitor and evaluator into a struct?
func (node *FileNode) VisitDepthChildFirst(visitor Visitor, evaluator VisitEvaluator, sorter OrderStrategy) error {
	if sorter == nil {
		sorter = GetSortOrderStrategy(ByName)
	}
	keys := sorter.orderKeys(node.Children)
	for _, name := range keys {
		child := node.Children[name]
		err := child.VisitDepthChildFirst(visitor, evaluator, sorter)
		if err != nil {
			return err
		}
	}
	// never visit the root node
	// if node == node.Tree.Root {
	// return nil
	// }
	if evaluator != nil && evaluator(node) || evaluator == nil {
		return visitor(node)
	}

	return nil
}

// VisitDepthParentFirst iterates a tree depth-first (starting at this FileNode),
// evaluating the shallowest depths first (visit while sinking down)
// TODO: understand why is this
// TODO: extract visitor and evaluator into a struct?
func (node *FileNode) VisitDepthParentFirst(visitor Visitor, evaluator VisitEvaluator, sorter OrderStrategy) error {
	var err error

	doVisit := evaluator != nil && evaluator(node) || evaluator == nil

	if !doVisit {
		return nil
	}

	// never visit the root node
	// if node != node.Tree.Root {
	// err = visitor(node)
	// if err != nil {
	// return err
	// }
	// }

	if sorter == nil {
		sorter = GetSortOrderStrategy(ByName)
	}
	keys := sorter.orderKeys(node.Children)
	for _, name := range keys {
		child := node.Children[name]
		err = child.VisitDepthParentFirst(visitor, evaluator, sorter)
		if err != nil {
			return err
		}
	}
	return err
}

// IsWhiteout returns an indication if this file may be an overlay-whiteout file.
func (node *FileNode) IsWhiteout() bool {
	return strings.HasPrefix(node.Name, whiteoutPrefix)
}

// IsLeaf returns true is the current node has no child nodes.
func (node *FileNode) IsLeaf() bool {
	return len(node.Children) == 0
}

// Path returns a slash-delimited string from the root of the greater tree to the current node
// (e.g. /a/path/to/here)
// TODO: why are the paths not set when parsing the .tar?
func (node *FileNode) Path() string {
	if node.path == "" {
		var path []string
		curNode := node
		for {
			if curNode.Parent == nil {
				break
			}

			name := curNode.Name
			if curNode == node {
				// white out prefixes are fictitious on leaf nodes
				name = strings.TrimPrefix(name, whiteoutPrefix)
			}

			path = append([]string{name}, path...)
			curNode = curNode.Parent
		}
		node.path = "/" + strings.Join(path, "/")
	}
	return strings.Replace(node.path, "//", "/", -1)
}

// deriveDiffType determines a DiffType to the current FileNode.
// Note: the DiffType of a node is always the DiffType of its attributes and its contents.
// The contents are the bytes of the file of the children of a directory.
func (node *FileNode) deriveDiffType(diffType DiffType) error {
	myDiffType := diffType
	for _, v := range node.Children {
		myDiffType = merge(myDiffType, v.Metadata.DiffType)
	}

	return node.AssignDiffType(myDiffType)
}

// AssignDiffType will assign the given DiffType to this node, possibly affecting child nodes.
func (node *FileNode) AssignDiffType(diffType DiffType) error {
	node.Metadata.DiffType = diffType

	if diffType == Removed {
		// if we've removed this node, then all children have been removed as well
		for _, child := range node.Children {
			err := child.AssignDiffType(Removed)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// compare the current node against the given node, returning a definitive DiffType.
func (node *FileNode) compare(other *FileNode) DiffType {
	if node == nil && other == nil {
		return Unmodified
	}

	if node == nil && other != nil {
		return Added
	}

	if node != nil && other == nil {
		return Removed
	}

	if other.IsWhiteout() {
		return Removed
	}
	if node.Name != other.Name {
		panic("comparing mismatched nodes")
	}

	return node.Metadata.FileInfo.Compare(other.Metadata.FileInfo)
}
