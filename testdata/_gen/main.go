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
