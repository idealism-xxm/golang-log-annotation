package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"logannotation"
	"os"
	"path/filepath"
	"strings"
)

var replace = false

func init() {
	flag.BoolVar(&replace, "replace", false, "使用生成的代码替换原本的代码")
}

func main() {
	flag.Parse()
	// 遍历需要应用注解的文件夹列表
	for _, dirPath := range flag.Args() {
		// 遍历每个文件夹下的所有文件/文件夹，并对每个元素执行 overwrite 函数
		if err := filepath.Walk(dirPath, walkFn); err != nil {
			panic(err)
		}
	}
}

// walkFn 函数会对每个 .go 文件处理，并调用注解处理器
func walkFn(path string, info os.FileInfo, err error) error {
	// 如果是文件夹，或者不是 .go 文件，则直接返回不处理
	if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
		return nil
	}

	fileSet, file, err := parseFile(path)
	// 如果注解修改了内容，则需要生成新的代码
	if logannotation.Overwrite(path, fileSet, file) {
		buf := &bytes.Buffer{}
		if err := format.Node(buf, fileSet, file); err != nil {
			panic(err)
		}

		// 如果不需要替换，则生成到另一个文件
		if !replace {
			lastSlashIndex := strings.LastIndex(path, "/")
			genDirPath := path[:lastSlashIndex] + "/_gen/"
			if err := os.Mkdir(genDirPath, 0755); err != nil && os.IsNotExist(err) {
				panic(err)
			}
			path = genDirPath + path[lastSlashIndex+1:]
		}

		if err := ioutil.WriteFile(path, buf.Bytes(), info.Mode()); err != nil {
			panic(err)
		}
	}

	return nil
}

func parseFile(filepath string) (*token.FileSet, *ast.File, error) {
	source, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, nil, err
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filepath, source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, nil, err
	}
	return fileSet, file, nil
}
