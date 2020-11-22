package logannotation

import (
	"fmt"
	"go/ast"
	"golang.org/x/tools/go/ast/astutil"
	"strconv"
	"strings"
)

var Handler = newHandler()

type handler struct{}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) Handle(info *Info) (modified bool, err error) {
	// log 注解只用于函数
	funcDecl, ok := info.Node.(*ast.FuncDecl)
	if !ok {
		return
	}

	// 如果没有注释，则直接处理下一个
	if funcDecl.Doc == nil {
		return
	}

	// 如果不是可以处理的注解，则直接返回
	doc := strings.Trim(funcDecl.Doc.Text(), "\t \n")
	if doc != "@Log()" {
		return
	}

	// 标记为已修改
	modified = true
	// 删除注释注解
	funcDecl.Doc.List = nil
	// 导入项目中 Logger 所在的包
	info.NamedImportAdder("", "logannotation/testdata/log")

	// 获取入参和出参（如果原来没有名字，则会设置一个默认的名字）
	paramNames := getFieldListNames(funcDecl.Type.Params)
	resultNames := getFieldListNames(funcDecl.Type.Results)

	// 构造打印函数的语句（项目中使用 logrus 打印日志）
	var loggerSelector = "log.Logger"
	// 如果第一个是 ctx ，则传入到 Logger 中，不进行打印
	if paramNames != nil && paramNames[0] == "ctx" {
		loggerSelector += ".WithContext(ctx)"
		paramNames = paramNames[1:]
	}
	// 将当前函数所在文件传给 Logger
	loggerSelector += fmt.Sprintf(".WithField(\"filepath\", \"%v\")", info.Filepath)
	// 我们这里仅用 Infof 打印，实际情况可根据 error 等信息自定义级别
	loggerSelector += ".Infof"

	// 生成进入函数时打印日志的语句
	funcDeclName := getFuncDeclName(funcDecl)
	beforeArgs := genBeforeLogArgs(funcDeclName, paramNames...)
	beforeExpr, err := NewCallExpr(loggerSelector, beforeArgs...)
	if err != nil {
		return false, err
	}

	// 生成进入函数时打印日志的语句
	afterArgs := genAfterLogArgs(funcDeclName, resultNames...)
	afterExpr, err := NewCallExpr(loggerSelector, afterArgs...)
	if err != nil {
		return false, err
	}

	toBeAddedStmts := []ast.Stmt{
		&ast.ExprStmt{X: beforeExpr},
		// 离开函数时的语句使用 defer 调用
		NewFuncLitDefer(&ast.ExprStmt{X: afterExpr}),
	}

	// 我们将添加的语句放在函数体最前面
	funcDecl.Body.List = append(toBeAddedStmts, funcDecl.Body.List...)

	return
}

func getFuncDeclName(funcDecl *ast.FuncDecl) string {
	// 是 function ，则直接使用函数名
	if funcDecl.Recv == nil {
		return funcDecl.Name.Name
	}

	// 是 method ，则还需把类型名称带上
	recvType := astutil.Unparen(funcDecl.Recv.List[0].Type)
	if starExpr, ok := recvType.(*ast.StarExpr); ok {
		recvType = starExpr.X
	}
	return fmt.Sprintf("%v.%v", recvType, funcDecl.Name.Name)
}

func genBeforeLogArgs(funcDeclName string, args ...string) []string {
	formatPrefix := fmt.Sprintf("#%v start, params: ", funcDeclName)
	return genLogArgs(formatPrefix, args...)
}

func genAfterLogArgs(funcDeclName string, args ...string) []string {
	formatPrefix := fmt.Sprintf("#%v end, results: ", funcDeclName)
	return genLogArgs(formatPrefix, args...)
}

func genLogArgs(formatPrefix string, args ...string) []string {
	// 如果没有参数，则使用 (void) 标识
	if len(args) == 0 {
		return []string{strconv.Quote(formatPrefix + "(void)")}
	}

	format := formatPrefix + strings.Repeat(", %+v", len(args))[2:]
	results := make([]string, len(args)+1)
	// 第一个参数是 format 字符串，所以需要用双引号包起来
	results[0] = strconv.Quote(format)
	for i, arg := range args {
		results[i+1] = arg
	}

	return results
}

func getFieldListNames(fieldList *ast.FieldList) []string {
	if fieldList == nil {
		return nil
	}

	// 先设置默认的名称，确保每个字段都有名字，方便等下打日志使用
	SetDefaultNames(fieldList.List...)
	return GetFieldNames(fieldList.List...)
}
