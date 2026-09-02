// Command zipdir 把一個目錄壓成 zip，並保留檔案的權限位元。
//
// 為什麼不用 zip(1)：發行包裡的 .app 只有在 Contents/MacOS 底下那個檔案帶著
// 執行位元時才點得開，而權限是存在 zip 的 external attributes 裡的。用
// archive/zip 明確設定，比多裝一個套件再相信它的預設值可靠。
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "用法：zipdir <來源目錄> <輸出.zip>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "zipdir:", err)
		os.Exit(1)
	}
}

func run(source, out string) error {
	source = filepath.Clean(source)
	// zip 內的路徑以來源目錄本身為根，解開之後才會得到同一個目錄而不是一堆檔案。
	root := filepath.Base(source)

	file, err := os.Create(out)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)

	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := root
		if relative != "." {
			name = root + "/" + filepath.ToSlash(relative)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		if entry.IsDir() {
			header.Name = strings.TrimSuffix(name, "/") + "/"
			_, err := archive.CreateHeader(header)
			return err
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		content, err := os.Open(path)
		if err != nil {
			return err
		}
		defer content.Close()
		_, err = io.Copy(writer, content)
		return err
	})
	if err != nil {
		_ = archive.Close()
		return err
	}
	return archive.Close()
}
