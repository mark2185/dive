package filetree

import (
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

func Values[K comparable, V comparable](m map[K]V) []V {
	res := make([]V, len(m))
	i := 0
	for _, v := range m {
		res[i] = v
		i++
	}
	return res
}

// TODO: move to the TUI package
const (
	newLine              = "\n"
	noBranchSpace        = "    "
	branchSpace          = "│   "
	middleItem           = "├─"
	lastItem             = "└─"
	whiteoutPrefix       = ".wh."
	doubleWhiteoutPrefix = ".wh..wh.."
	uncollapsedItem      = "─ "
	collapsedItem        = "⊕ "
)

// FileTree represents a set of files, directories, and their relations.
type FileTree struct {
	Root     *FileNode
	Count    uint   // total number of nodes in the tree (dirs, files, symlinks)
	FileSize uint64 // sum of the sizes of all nodes
	Name     string // TODO: ?
	// Id        uuid.UUID // TODO: not used at all?
	SortOrder SortOrder // nodes sort order when displaying the tree
}

// NewFileTree creates an empty FileTree
func NewFileTree() *FileTree {
	root := FileNode{
		Children: map[string]*FileNode{},
	}
	return &FileTree{
		Count: 1,
		Root:  &root,
		//Id:        uuid.New(),
		SortOrder: ByName,
	}
}

// renderParams is a representation of a FileNode in the context of the greater tree. All
// data stored is necessary for rendering a single line in a tree format.
type renderParams struct {
	node          *FileNode
	spaces        []bool
	childSpaces   []bool
	showCollapsed bool
	isLast        bool
}

type FileNodeMeta struct {
	Node  *FileNode
	Depth int
}

func (tree *FileTree) Sort() []*FileNodeMeta {
	sorter := GetSortOrderStrategy(tree.SortOrder)

	res := []*FileNodeMeta{}
	nodesToVisit := []*FileNodeMeta{{Node: tree.Root, Depth: 0}}
	var head *FileNodeMeta
	for {
		if len(nodesToVisit) == 0 {
			break
		}
		head, nodesToVisit = nodesToVisit[0], nodesToVisit[1:]
		if head != nil {
			res = append(res, head)
			children := []*FileNodeMeta{}
			for _, child := range sorter.orderKeys(head.Node.Children) {
				children = append(children, &FileNodeMeta{head.Node.Children[child], head.Depth + 1})
			}
			nodesToVisit = append(children, nodesToVisit...)
		}
	}
	return res
}

// renderStringTreeBetween returns a string representing the given tree between the given rows. Since each node
// is rendered on its own line, the returned string shows the visible nodes not affected by a collapsed parent.
func (tree *FileTree) renderStringTreeBetween(startRow, stopRow int, showAttributes bool) string {
	// generate a list of nodes to render
	var params = []renderParams{}
	var result string

	// visit from the front of the list
	var paramsToVisit = []renderParams{{node: tree.Root, spaces: []bool{}, showCollapsed: false, isLast: false}}
	for currentRow := 0; len(paramsToVisit) > 0 && currentRow <= stopRow; currentRow++ {
		// pop the first node
		var currentParams renderParams
		currentParams, paramsToVisit = paramsToVisit[0], paramsToVisit[1:]

		// take note of the next nodes to visit later
		sorter := GetSortOrderStrategy(tree.SortOrder)
		keys := sorter.orderKeys(currentParams.node.Children)

		var childParams = make([]renderParams, 0)
		for idx, name := range keys {
			child := currentParams.node.Children[name]
			// don't visit this node...
			if child.Metadata.ViewInfo.Hidden || currentParams.node.Metadata.ViewInfo.Collapsed {
				continue
			}

			// visit this node...
			isLast := idx == (len(currentParams.node.Children) - 1)
			showCollapsed := child.Metadata.ViewInfo.Collapsed && len(child.Children) > 0

			// completely copy the reference slice
			childSpaces := make([]bool, len(currentParams.childSpaces))
			copy(childSpaces, currentParams.childSpaces)

			if len(child.Children) > 0 && !child.Metadata.ViewInfo.Collapsed {
				childSpaces = append(childSpaces, isLast)
			}

			childParams = append(childParams, renderParams{
				node:          child,
				spaces:        currentParams.childSpaces,
				childSpaces:   childSpaces,
				showCollapsed: showCollapsed,
				isLast:        isLast,
			})
		}
		// keep the child nodes to visit later
		paramsToVisit = append(childParams, paramsToVisit...)

		// never process the root node
		if currentParams.node == tree.Root {
			currentRow--
			continue
		}

		// process the current node
		if currentRow >= startRow && currentRow <= stopRow {
			params = append(params, currentParams)
		}
	}

	// render the result
	for idx := range params {
		currentParams := params[idx]

		if showAttributes {
			result += currentParams.node.MetadataString() + " "
		}
		result += currentParams.node.renderTreeLine(currentParams.spaces, currentParams.isLast, currentParams.showCollapsed)
	}

	return result
}

func (tree *FileTree) VisibleSize() int {
	var size int

	visitor := func(node *FileNode) error {
		size++
		return nil
	}
	visitEvaluator := func(node *FileNode) bool {
		if node.Metadata.FileInfo.IsDir {
			// we won't visit a collapsed dir, but we need to count it
			if node.Metadata.ViewInfo.Collapsed {
				size++
			}
			return !node.Metadata.ViewInfo.Collapsed && !node.Metadata.ViewInfo.Hidden
		}
		return !node.Metadata.ViewInfo.Hidden
	}
	err := tree.VisitDepthParentFirst(visitor, visitEvaluator)
	if err != nil {
		logrus.Errorf("unable to determine visible tree size: %+v", err)
	}

	// don't include root
	size--

	return size
}

// String returns the entire tree in an ASCII representation.
func (tree *FileTree) String(showAttributes bool) string {
	return tree.renderStringTreeBetween(0, int(tree.Count), showAttributes)
}

// StringBetween returns a partial tree in an ASCII representation.
func (tree *FileTree) StringBetween(start, stop int, showAttributes bool) string {
	return tree.renderStringTreeBetween(start, stop, showAttributes)
}

// Copy returns a copy of the given FileTree
func (tree *FileTree) Copy() *FileTree {
	newTree := NewFileTree()
	newTree.Count = tree.Count
	newTree.FileSize = tree.FileSize
	newTree.Root = tree.Root.Copy(newTree.Root)
	newTree.SortOrder = tree.SortOrder

	// update the tree pointers
	err := newTree.VisitDepthChildFirst(func(node *FileNode) error {
		return nil
	}, nil)

	if err != nil {
		logrus.Errorf("unable to propagate tree on copy(): %+v", err)
	}

	return newTree
}

// Visitor is a function that processes, observes, or otherwise transforms the given node
type Visitor func(*FileNode) error

// VisitEvaluator is a function that indicates whether the given node should be visited by a Visitor.
type VisitEvaluator func(*FileNode) bool

// VisitDepthChildFirst iterates the given tree depth-first, evaluating the deepest depths first (visit on bubble up)
func (tree *FileTree) VisitDepthChildFirst(visitor Visitor, evaluator VisitEvaluator) error {
	sorter := GetSortOrderStrategy(tree.SortOrder)
	return tree.Root.VisitDepthChildFirst(visitor, evaluator, sorter)
}

// VisitDepthParentFirst iterates the given tree depth-first, evaluating the shallowest depths first (visit while sinking down)
func (tree *FileTree) VisitDepthParentFirst(visitor Visitor, evaluator VisitEvaluator) error {
	sorter := GetSortOrderStrategy(tree.SortOrder)
	return tree.Root.VisitDepthParentFirst(visitor, evaluator, sorter)
}

// Stack takes two trees and combines them together. This is done by "stacking" the given tree on top of the owning tree.
func (tree *FileTree) Stack(upper *FileTree) (failed []PathError, stackErr error) {
	graft := func(node *FileNode) error {
		if node.IsWhiteout() {
			err := tree.RemovePath(node.Path())
			if err != nil {
				failed = append(failed, NewPathError(node.Path(), ActionAdd, err))
			}
		} else {
			_, _, err := tree.AddPath(node.Path(), node.Metadata.FileInfo)
			if err != nil {
				failed = append(failed, NewPathError(node.Path(), ActionRemove, err))
			}
		}
		return nil
	}
	stackErr = upper.VisitDepthChildFirst(graft, nil)
	return failed, stackErr
}

// GetNode fetches a single node when given a slash-delimited string from root ('/') to the desired node (e.g. '/a/node/path')
func (tree *FileTree) GetNode(path string) (*FileNode, error) {
	// TODO: is trim really needed?
	// nodeNames := strings.Split(strings.Trim(path, "/"), "/")
	nodeNames := strings.Split(path, "/")
	node := tree.Root
	for _, name := range filterEmpty(nodeNames) {
		if node.Children[name] == nil {
			return nil, fmt.Errorf("path does not exist: %s", path)
		}
		node = node.Children[name]
	}
	return node, nil
}

// Filters out empty strings from a list
// TODO: move to utils
func filterEmpty(l []string) []string {
	res := []string{}
	for _, elem := range l {
		if len(elem) > 0 {
			res = append(res, elem)
		}
	}
	return res
}

// Adds a new node to the tree with the given payload
func (tree *FileTree) AddPath(filepath string, data FileInfo) (*FileNode, []*FileNode, error) {
	filepath = path.Clean(filepath)
	if filepath == "." {
		return nil, nil, fmt.Errorf("cannot add relative path '%s'", filepath)
	}

	nodeNames := strings.Split(strings.Trim(filepath, "/"), "/")
	node := tree.Root
	addedNodes := []*FileNode{}
	for _, name := range filterEmpty(nodeNames) {
		// find or create node
		if node.Children[name] != nil {
			node = node.Children[name]
		} else if strings.HasPrefix(name, doubleWhiteoutPrefix) {
			// don't add paths that should be deleted
			return nil, addedNodes, nil
		} else {
			// don't attach the payload. The payload is destined for the
			// Path's end node, not any intermediary node.
			node = node.AddChild(name, FileInfo{})
			if node == nil {
				// the child could not be added
				return node, addedNodes, fmt.Errorf(fmt.Sprintf("could not add child node: '%s' (path:'%s')", name, filepath))
			}

			tree.Count++
			addedNodes = append(addedNodes, node)
		}
	}

	// attach payload to the last specified node
	node.Metadata.FileInfo = data
	node.Size = data.Size

	return node, addedNodes, nil
}

// RemovePath removes a node from the tree given its path.
func (tree *FileTree) RemovePath(path string) error {
	node, err := tree.GetNode(path)
	if err != nil {
		return err
	}
	if node == tree.Root {
		return fmt.Errorf("cannot remove the tree root")
	}
	// TODO: fix the tree.Count when removing nodes
	// what happens if a directory is removed?
	return node.Remove()
}

type compareMark struct {
	lowerNode *FileNode
	upperNode *FileNode
	tentative DiffType
	final     DiffType
}

// CompareAndMark marks the FileNodes in the owning (lower) tree with DiffType annotations when compared to the given (upper) tree.
func (tree *FileTree) CompareAndMark(upper *FileTree) ([]PathError, error) {
	// always compare relative to the original, unaltered tree.
	originalTree := tree

	modifications := make([]compareMark, 0)
	failed := make([]PathError, 0)

	graft := func(upperNode *FileNode) error {
		if upperNode.IsWhiteout() {
			err := tree.markRemoved(upperNode.Path())
			if err != nil {
				failed = append(failed, NewPathError(upperNode.Path(), ActionRemove, err))
			}
			return nil
		}

		// note: since we are not comparing against the original tree (copying the tree is expensive) we may mark the parent
		// of an added node incorrectly as modified. This will be corrected later.
		originalLowerNode, _ := originalTree.GetNode(upperNode.Path())

		if originalLowerNode == nil {
			_, newNodes, err := tree.AddPath(upperNode.Path(), upperNode.Metadata.FileInfo)
			if err != nil {
				failed = append(failed, NewPathError(upperNode.Path(), ActionAdd, err))
				return nil
			}
			for idx := len(newNodes) - 1; idx >= 0; idx-- {
				newNode := newNodes[idx]
				modifications = append(modifications, compareMark{lowerNode: newNode, upperNode: upperNode, tentative: -1, final: Added})
			}
			return nil
		}

		// the file exists in the lower layer
		lowerNode, _ := tree.GetNode(upperNode.Path())
		diffType := lowerNode.compare(upperNode)
		modifications = append(modifications, compareMark{lowerNode: lowerNode, upperNode: upperNode, tentative: diffType, final: -1})

		return nil
	}
	// we must visit from the leaves upwards to ensure that diff types can be derived from and assigned to children
	err := upper.VisitDepthChildFirst(graft, nil)
	if err != nil {
		return failed, err
	}

	// take note of the comparison results on each note in the owning tree.
	for _, pair := range modifications {
		if pair.final > 0 {
			err = pair.lowerNode.AssignDiffType(pair.final)
			if err != nil {
				return failed, err
			}
		} else if pair.lowerNode.Metadata.DiffType == Unmodified {
			err = pair.lowerNode.deriveDiffType(pair.tentative)
			if err != nil {
				return failed, err
			}
		}

		// persist the upper's payload on the owning tree
		pair.lowerNode.Metadata.FileInfo = *pair.upperNode.Metadata.FileInfo.Copy()
	}
	return failed, nil
}

// markRemoved annotates the FileNode at the given path as Removed.
func (tree *FileTree) markRemoved(path string) error {
	node, err := tree.GetNode(path)
	if err != nil {
		return err
	}
	return node.AssignDiffType(Removed)
}

// StackTreeRange combines an array of trees into a single tree
func StackTreeRange(trees []*FileTree, start, stop int) (*FileTree, []PathError, error) {
	errors := make([]PathError, 0)
	tree := trees[0].Copy()
	for idx := start; idx <= stop; idx++ {
		failedPaths, err := tree.Stack(trees[idx])
		if len(failedPaths) > 0 {
			errors = append(errors, failedPaths...)
		}
		if err != nil {
			logrus.Errorf("could not stack tree range: %v", err)
			return nil, nil, err
		}
	}
	return tree, errors, nil
}
