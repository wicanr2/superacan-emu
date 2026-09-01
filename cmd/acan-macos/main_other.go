//go:build !darwin

// acan-macos 是 macOS 前端。它只在 darwin 上建置得出可執行的東西；
// 在其他平台留一個明確的入口，讓 go build ./... 不會因為空的 main 套件失敗，
// 也讓誤用的人看到原因而不是一個奇怪的連結錯誤。
package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Fprintf(os.Stderr, "acan-macos 只能在 macOS 上執行，目前是 %s/%s\n", runtime.GOOS, runtime.GOARCH)
	os.Exit(1)
}
