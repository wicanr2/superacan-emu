// Command macho 是 macOS 發行包用的兩件小事：把單架構的 Mach-O 合成 universal
// binary，以及做交叉編譯之後只能靠靜態檢查補的那幾項驗收。
//
// 為什麼不用 lipo／otool：本專案的 macOS 執行檔是 CGO_ENABLED=0 的純 Go，
// Go 自己就產 Mach-O，整條路上沒有 Apple 的工具鏈。為了兩個檔案格式操作把
// osxcross 或 cctools 搬進來不划算，而這兩件事本身很小——fat binary 是一個
// header 加幾個對齊過的切片，load command 用標準庫的 debug/macho 就讀得到。
//
//	macho fat  <輸出> <輸入...>
//	macho check <檔案>
package main

import (
	"debug/macho"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// fatMagic 是 universal binary 的魔數，big-endian 寫入。
const fatMagic = 0xcafebabe

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "用法：macho fat <輸出> <輸入...> ｜ macho check <檔案>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "fat":
		err = writeFat(os.Args[2], os.Args[3:])
	case "check":
		err = check(os.Args[2])
	default:
		err = fmt.Errorf("不認得的子命令 %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "macho:", err)
		os.Exit(1)
	}
}

type slice struct {
	cpu     macho.Cpu
	subtype uint32
	align   uint32
	data    []byte
}

// writeFat 組出 universal binary。每個切片依自己的頁大小對齊：arm64 是 16 KB
// （2^14），x86_64 是 4 KB（2^12）。對齊寫錯的話 dyld 會拒絕載入，而檔案本身
// 看起來完全正常。
func writeFat(out string, inputs []string) error {
	if len(inputs) < 2 {
		return fmt.Errorf("至少要兩個架構才需要合併")
	}
	slices := make([]slice, 0, len(inputs))
	for _, path := range inputs {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := macho.NewFile(newReaderAt(data))
		if err != nil {
			return fmt.Errorf("%s：%w", path, err)
		}
		align := uint32(12)
		if file.Cpu == macho.CpuArm64 {
			align = 14
		}
		slices = append(slices, slice{
			cpu: file.Cpu, subtype: file.SubCpu, align: align, data: data,
		})
	}

	header := make([]byte, 8+20*len(slices))
	binary.BigEndian.PutUint32(header[0:4], fatMagic)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(slices)))

	offset := uint32(len(header))
	offsets := make([]uint32, len(slices))
	for index, s := range slices {
		step := uint32(1) << s.align
		offset = (offset + step - 1) / step * step
		offsets[index] = offset
		entry := header[8+20*index:]
		binary.BigEndian.PutUint32(entry[0:4], uint32(s.cpu))
		binary.BigEndian.PutUint32(entry[4:8], s.subtype)
		binary.BigEndian.PutUint32(entry[8:12], offset)
		binary.BigEndian.PutUint32(entry[12:16], uint32(len(s.data)))
		binary.BigEndian.PutUint32(entry[16:20], s.align)
		offset += uint32(len(s.data))
	}

	file, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(header); err != nil {
		return err
	}
	for index, s := range slices {
		if _, err := file.Seek(int64(offsets[index]), io.SeekStart); err != nil {
			return err
		}
		if _, err := file.Write(s.data); err != nil {
			return err
		}
	}
	fmt.Printf("%s：%d 個架構\n", out, len(slices))
	return nil
}

// check 是交叉編譯之後的靜態驗收。Linux 上執行不了 macOS 執行檔，所以
// 「編得出來」與「跑得起來」之間這一段只能這樣補。
func check(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var files []*macho.File
	if fat, err := macho.NewFatFile(newReaderAt(data)); err == nil {
		for index := range fat.Arches {
			files = append(files, fat.Arches[index].File)
		}
	} else {
		file, err := macho.NewFile(newReaderAt(data))
		if err != nil {
			return err
		}
		files = append(files, file)
	}

	failed := false
	for _, file := range files {
		arch := file.Cpu.String()
		signed, minOS := scanLoads(file)
		deps := dylibs(file)
		fmt.Printf("%s  簽章=%v  最低系統=%s  相依=%v\n", arch, signed, minOS, deps)

		// Apple 從 arm64 開始強制簽章：沒有簽章的 arm64 執行檔在 Apple Silicon
		// 上會被核心直接殺掉（Killed: 9），而檔案格式完全正常，在 Linux 這端
		// 看不出任何異狀。x86_64 沒有這個限制。
		if file.Cpu == macho.CpuArm64 && !signed {
			fmt.Fprintf(os.Stderr, "  ※ arm64 缺 LC_CODE_SIGNATURE，在 Apple Silicon 上會被殺掉\n")
			failed = true
		}
		// 相依只能落在系統路徑：連到編譯機才有的 dylib，使用者的 Mac 上不存在。
		for _, dep := range deps {
			if !isSystemPath(dep) {
				fmt.Fprintf(os.Stderr, "  ※ 相依 %s 不在系統路徑，使用者的 Mac 上不會有\n", dep)
				failed = true
			}
		}
	}
	if failed {
		return fmt.Errorf("靜態驗收未通過")
	}
	return nil
}

func isSystemPath(name string) bool {
	return len(name) >= 9 && name[:9] == "/usr/lib/" ||
		len(name) >= 16 && name[:16] == "/System/Library/"
}

// scanLoads 找 LC_CODE_SIGNATURE 與 LC_BUILD_VERSION。debug/macho 沒有把這兩個
// 解成型別，所以直接讀原始位元組。
func scanLoads(file *macho.File) (signed bool, minOS string) {
	const (
		cmdCodeSignature = 0x1d
		cmdBuildVersion  = 0x32
	)
	minOS = "未標示"
	for _, load := range file.Loads {
		raw := load.Raw()
		if len(raw) < 8 {
			continue
		}
		switch file.ByteOrder.Uint32(raw[0:4]) {
		case cmdCodeSignature:
			signed = true
		case cmdBuildVersion:
			if len(raw) >= 20 {
				minOS = version(file.ByteOrder.Uint32(raw[16:20]))
			}
		}
	}
	return signed, minOS
}

// version 把 LC_BUILD_VERSION 的 xxxx.yy.zz 編碼還原成字串。
func version(value uint32) string {
	return fmt.Sprintf("%d.%d.%d", value>>16, (value>>8)&0xff, value&0xff)
}

func dylibs(file *macho.File) []string {
	var out []string
	for _, load := range file.Loads {
		if dylib, ok := load.(*macho.Dylib); ok {
			out = append(out, dylib.Name)
		}
	}
	return out
}

type readerAt []byte

func newReaderAt(data []byte) io.ReaderAt { return readerAt(data) }

func (r readerAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(r)) {
		return 0, io.EOF
	}
	n := copy(p, r[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
