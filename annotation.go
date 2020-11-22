package logannotation

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

// Overwrite 会对每个 file 处理，运行注册的注解 handler ，并返回其是否被修改
func Overwrite(filepath string, fileSet *token.FileSet, file *ast.File) (modified bool) {
	// 初始化处理本次文件所需的信息对象
	info := &Info{
		Filepath: filepath,
		NamedImportAdder: func(name string, path string) bool {
			return astutil.AddNamedImport(fileSet, file, name, path)
		},
	}

	// 遍历当前文件 ast 上的所有节点
	astutil.Apply(file, nil, func(cursor *astutil.Cursor) bool {
		// 处理 log 注解
		info.Node = cursor.Node()
		nodeModified, err := Handler.Handle(info)
		if err != nil {
			panic(err)
		}

		if nodeModified {
			modified = nodeModified
		}

		return true
	})

	return
}
