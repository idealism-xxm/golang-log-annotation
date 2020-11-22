package logannotation

import "go/ast"

type NamedImportAdderFunc func(name string, path string) (added bool)

type Info struct {
	// 当前需要处理的节点所在文件路径
	Filepath string
	// 当前需要处理的节点
	Node ast.Node
	// 用于处理时添加一个命名的 import
	NamedImportAdder NamedImportAdderFunc
}
