## 背景

前一段时间线上出现了一个问题：在压测后偶尔会出现一台机器查询数据无结果但是没有返回 `err` 的情况，导致后续处理都出错。由于当时我们仅在最外层打印了 `err` ，没有打印入参和出参，所以导致很难排查问题到底出现在哪一环节。

经过艰难地排查出问题后，感觉需要在代码里添加打印关键函数的入参和出参数，但这个逻辑都是重复的，也不想将这一逻辑侵入开发流程，所以就想到了代码生成方式的注释注解。即我们提前定义好一个注释注解（例如：`// @Log()`），并且在 Docker 中编译前运行代码生成的逻辑，将所有拥有该注释注解的函数进行修改，在函数体前面添加打印入参和出参的逻辑。这样就不需要让日志打印侵入到业务代码中，并且后续可以很方便替换成其他的打印逻辑（例如根据`@Log` 内的参数或者返回值等自定义日志级别）。

## 编写代码

我们可以使用 AST 的方式去解析、识别并修改代码， `go/ast` 已经提供了相应的功能，我们查看我们关心的节点部分及其相关的信息结构，可以使用 [goast-viewer](https://yuroyoro.github.io/goast-viewer/index.html) 直接查看 AST ，当然也可以本地进行调试。

### 遍历 `.go` 文件

首先需要使用 `filepath.Walk` 函数遍历指定文件夹下的所有文件，对每一个文件都会执行传入的 `walkFn` 函数。  `walkFn` 函数会将 `.go` 文件解析成 AST ，将其交由注释注解的处理器处理，然后根据是否修改了 AST 决定是否生成新的代码。

```go
// walkFn 函数会对每个 .go 文件处理，并调用注解处理器
func walkFn(path string, info os.FileInfo, err error) error {
	// 如果是文件夹，或者不是 .go 文件，则直接返回不处理
	if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
		return nil
	}

	// 将 .go 文件解析成 AST
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
			genDirPath :=path[:lastSlashIndex] + "/_gen/"
			if err := os.Mkdir(genDirPath, 0755); err != nil && os.IsNotExist(err){
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
```

### 遍历 AST

当注释注解处理器拿到 AST 后，就需要使用 `astutil.Apply` 函数遍历整颗 AST ，并对每个节点进行处理，同时为了方便修改时添加 `import` ，我们包一层函数供内部调用，并把一些关键信息打包在一起。

```go
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
```

### 识别注释注解

接下来我们就需要识别注释注解，跳过不相关的节点，示例中不做额外处理，仅当注释为 `@Log()` 才认为需要处理，可以根据需要添加相应的逻辑。

```go
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
	...
}
```

### 获取函数入参和出参

首先我们需要获取函数的入参和出参，这里我们以出参举例。出参定义在 `funcDecl.Type.Results` ，并且可能没有指定名称，所以需要先为以 `_0`, `_1`, `...` 这样的形式为没有名称的变量设置默认名称，然后按照顺序获取所有变量的名称列表。

```go
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
```

### 获取打印语句

假设我们所需的打印语句为： `log.Logger.WithContext(ctx).WithField("filepath", filepath).Infof(format, arg0, arg1)` ，那么函数选择器的表达式可以直接使用 `parser.ParseExpr` 函数生成，其中的参数 `(format, arg0, arg1)` 手动拼接即可。

```go
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
```

由于出参需要等函数执行完毕后执行，所以打印出参的语句还需要放在 `defer` 函数内执行。

```go
// NewFuncLitDefer 产生一个 defer 语句，运行一个匿名函数，函数体是入参语句列表
func NewFuncLitDefer(funcStmts ...ast.Stmt) *ast.DeferStmt {
	return &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: NewFuncLit(&ast.FuncType{}, funcStmts...),
		},
	}
}
```

### 修改函数体

至此我们已经获得了打印入参和出参的语句，接下来就是把他们放在原本函数体的最前面，保证开始和结束时执行。

```go
toBeAddedStmts := []ast.Stmt{
    &ast.ExprStmt{X: beforeExpr},
    // 离开函数时的语句使用 defer 调用
    NewFuncLitDefer(&ast.ExprStmt{X: afterExpr}),
}

// 我们将添加的语句放在函数体最前面
funcDecl.Body.List = append(toBeAddedStmts, funcDecl.Body.List...)
```

## 运行

为了测试我们的注释注解是否工作正确，我们使用如下代码进行测试：

```go
package main

import (
	"context"
	"logannotation/testdata/log"
)

func main() {
	fn(context.Background(), 1, "2", "3", true)
}

// @Log()
func fn(ctx context.Context, a int, b, c string, d bool) (int, string, string) {
	log.Logger.WithContext(ctx).Infof("#fn executing...")
	return a, b, c
}
```

运行 `go run logannotation/cmd/generator /Users/idealism/Workspaces/Go/golang-log-annotation/testdata` 执行代码生成，在 `/Users/idealism/Workspaces/Go/golang-log-annotation/testdata/_gen` 下可找到生成的代码：

```go
package main

import (
	"context"
	"logannotation/testdata/log"
)

func main() {
	fn(context.Background(), 1, "2", "3", true)
}

func fn(ctx context.Context, a int, b, c string, d bool) (_0 int, _1 string, _2 string) {
	log.Logger.WithContext(
		ctx).WithField("filepath", "/Users/idealism/Workspaces/Go/golang-log-annotation/testdata/main.go").Infof("#fn start, params: %+v, %+v, %+v, %+v", a, b, c, d)
	defer func() {
		log.Logger.WithContext(
			ctx).WithField("filepath", "/Users/idealism/Workspaces/Go/golang-log-annotation/testdata/main.go").Infof("#fn end, results: %+v, %+v, %+v", _0, _1, _2)
	}()

	log.Logger.WithContext(ctx).Infof("#fn executing...")
	return a, b, c
}
```

可以看到已经按照我们的想法正确生成了代码，并且运行后能按照正确的顺序打印正确的入参和出参。实际使用时会在 `ctx` 中加入 apm 的 `traceId` ，并且在 `logrus` 的 `Hooks` 中将其在打印前放入到 `Fields` 中，这样搜索的时候可以将同一请求的所有日志聚合在一起。

```shell script
INFO[0000] #fn start, params: 1, 2, 3, true              filepath=/Users/idealism/Workspaces/Go/golang-log-annotation/testdata/main.go
INFO[0000] #fn executing...                             
INFO[0000] #fn end, results: 1, 2, 3                     filepath=/Users/idealism/Workspaces/Go/golang-log-annotation/testdata/main.go
```

## 扩展

以上代码是一种简单方式地定制化处理注解，仅处理了打印日志这一逻辑，当然还存在更多扩展的可能性和优化。

- 注册自定义注解（这样可以把更多重复逻辑抽出来，例如：参数校验、缓存等逻辑）
- 同时使用多个注解
- 注解解析成语法树，支持注解参数
- 生成的代码仅在需要时换行
