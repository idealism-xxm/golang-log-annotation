package logannotation

import (
	"fmt"
	"go/ast"
	"go/parser"
)

// NewIdents 按顺序生成每个名称的 ast.Ident
func NewIdents(names ...string) []*ast.Ident {
	idents := make([]*ast.Ident, len(names))
	for i, name := range names {
		idents[i] = ast.NewIdent(name)
	}

	return idents
}

// GetIdentNames 按顺序返回 idents 的名称列表
func GetIdentNames(idents ...*ast.Ident) []string {
	names := make([]string, len(idents))
	for i, ident := range idents {
		names[i] = ident.Name
	}

	return names
}

// SetDefaultNames 给没有名字的 Field 设置默认的名称
// 默认名称格式： _0, _1, ...
// true: 表示至少设置了一个名称
// false: 表示未设置过名称
func SetDefaultNames(fields ...*ast.Field) bool {
	index := 0
	for _, field := range fields {
		if field.Names == nil {
			field.Names = NewIdents(fmt.Sprintf("_%v", index))
			index++
		}
	}

	return index > 0
}

// GetFieldNames 按顺序返回 fields 的名称列表
// 如果一个 Field 有多个名称，那么它们会在一起
// (a int, b, c string, d bool) 会返回 ("a", "b", "c", "d")
func GetFieldNames(fields ...*ast.Field) []string {
	var names []string
	for _, field := range fields {
		names = append(names, GetIdentNames(field.Names...)...)
	}

	return names
}

// NewCallExpr 产生一个调用表达式
// 待产生表达式： log.Logger.WithContext(ctx).Infof(arg0, arg1)
// 其中：
//	funcSelector = "log.Logger.WithContext(ctx).Infof"
// 	args = ("arg0", "arg1")
// 调用语句：NewCallExpr("log.Logger.WithContext(ctx).Infof", "arg0", "arg1")
func NewCallExpr(funcSelector string, args ...string) (*ast.CallExpr, error) {
	// 获取函数对应的表达式
	funcExpr, err := parser.ParseExpr(funcSelector)
	if err != nil {
		return nil, err
	}

	// 组装参数列表
	argsExpr := make([]ast.Expr, len(args))
	for i, arg := range args {
		argsExpr[i] = ast.NewIdent(arg)
	}

	return &ast.CallExpr{
		Fun:  funcExpr,
		Args: argsExpr,
	}, nil
}

// NewFuncLitDefer 产生一个 defer 语句，运行一个匿名函数，函数体是入参语句列表
func NewFuncLitDefer(funcStmts ...ast.Stmt) *ast.DeferStmt {
	return &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: NewFuncLit(&ast.FuncType{}, funcStmts...),
		},
	}
}

// NewFuncLit 产生一个匿名函数
func NewFuncLit(funcType *ast.FuncType, stmts ...ast.Stmt) *ast.FuncLit {
	return &ast.FuncLit{
		Type: funcType,
		Body: &ast.BlockStmt{
			List: stmts,
		},
	}
}
