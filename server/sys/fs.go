package sys

import (
	"os"
	"path/filepath"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

// package containing fs related operation

// Build a directory tree started from the specified path using DFS.
// Then return the flattened tree represented as a list.
func DirectoryTree() (*[]string, error) {
	type Node struct {
		path     string
		children []Node
	}

	var (
		rootPath = config.Instance().Paths.DownloadPath

		stack     = internal.NewStack[Node]()
		flattened = make([]string, 0)
	)

	stack.Push(Node{path: rootPath})

	flattened = append(flattened, rootPath)

	for stack.IsNotEmpty() {
		current := stack.Pop().Value

		children, err := os.ReadDir(current.path)
		if err != nil {
			return nil, err
		}
		for _, entry := range children {
			var (
				childPath = filepath.Join(current.path, entry.Name())
				childNode = Node{path: childPath}
			)
			if entry.IsDir() {
				current.children = append(current.children, childNode)
				stack.Push(childNode)
				flattened = append(flattened, childNode.path)
			}
		}
	}
	return &flattened, nil
}
