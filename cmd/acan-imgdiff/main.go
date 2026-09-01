// acan-imgdiff 比較 oracle 截圖與本專案 renderer 輸出的 PNG。
//
// 兩張圖必須同尺寸；工具只做逐像素比較，不做縮放、濾波或色彩空間轉換，
// 因為 oracle（Bcan 0.0.8b）與本專案都直接輸出 UM6618 顯示孔徑的原生像素。
// 給定目錄時會逐檔比較並列出最接近的候選，用來在不確定 frame 對齊時定位。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
)

type comparison struct {
	name      string
	mismatch  int
	total     int
	meanError float64
	maxError  int
}

func main() {
	reference := flag.String("reference", "", "oracle PNG")
	candidate := flag.String("candidate", "", "single candidate PNG")
	candidateDir := flag.String("candidate-dir", "", "directory of candidate PNGs; report the closest ones")
	top := flag.Int("top", 5, "with --candidate-dir, how many closest candidates to report")
	diffOut := flag.String("diff", "", "write a difference mask PNG for the best candidate")
	listDifferences := flag.Int("list-differences", 0, "print this many differing pixels of the best candidate as reference/candidate RGB pairs")
	activeWidth := flag.Int("width", 0, "compare only the leftmost N columns; 0 compares the full image")
	flag.Parse()

	if *reference == "" || (*candidate == "" && *candidateDir == "") {
		fail("--reference and one of --candidate/--candidate-dir are required")
	}

	referenceImage := loadPNG(*reference)
	var names []string
	if *candidate != "" {
		names = append(names, *candidate)
	}
	if *candidateDir != "" {
		entries, err := os.ReadDir(*candidateDir)
		if err != nil {
			fail(fmt.Sprintf("read candidate directory: %v", err))
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" {
				continue
			}
			names = append(names, filepath.Join(*candidateDir, entry.Name()))
		}
	}
	if len(names) == 0 {
		fail("no candidate PNG found")
	}

	results := make([]comparison, 0, len(names))
	for _, name := range names {
		results = append(results, compare(referenceImage, loadPNG(name), name, *activeWidth))
	}
	// 以平均通道誤差排序：畫面幾乎一定有整張不同的候選，mismatch 會全部是 100%，
	// 只有平均誤差能分辨「接近但有幾處錯」與「完全不同的畫面」。
	sort.SliceStable(results, func(i, j int) bool { return results[i].meanError < results[j].meanError })

	limit := *top
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	for _, result := range results[:limit] {
		fmt.Printf("candidate=%s mismatch=%d/%d (%.2f%%) mean_error=%.3f max_error=%d\n",
			result.name, result.mismatch, result.total,
			100*float64(result.mismatch)/float64(result.total), result.meanError, result.maxError)
	}

	if *listDifferences > 0 {
		printDifferences(referenceImage, loadPNG(results[0].name), *listDifferences)
	}
	if *diffOut != "" {
		writeDiff(*diffOut, referenceImage, loadPNG(results[0].name))
	}
}

// compare 逐像素比對 RGB 三通道；alpha 不比較，因為兩端來源的 alpha 語意不同。
// activeWidth 大於零時只比較最左邊那些欄：UM6618 在 256 模式下的顯示孔徑比
// oracle 截圖的固定 320 欄窄，右側欄位在兩邊沒有相同語意，不應計入差異。
func compare(reference, candidate image.Image, name string, activeWidth int) comparison {
	bounds := reference.Bounds()
	if candidate.Bounds() != bounds {
		fail(fmt.Sprintf("%s has bounds %v, reference has %v", name, candidate.Bounds(), bounds))
	}
	maxX := bounds.Max.X
	if activeWidth > 0 && bounds.Min.X+activeWidth < maxX {
		maxX = bounds.Min.X + activeWidth
	}
	result := comparison{name: name, total: (maxX - bounds.Min.X) * bounds.Dy()}
	var accumulated float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < maxX; x++ {
			r0, g0, b0 := channels(reference.At(x, y))
			r1, g1, b1 := channels(candidate.At(x, y))
			difference := absDifference(r0, r1) + absDifference(g0, g1) + absDifference(b0, b1)
			if difference != 0 {
				result.mismatch++
			}
			for _, single := range [3]int{absDifference(r0, r1), absDifference(g0, g1), absDifference(b0, b1)} {
				if single > result.maxError {
					result.maxError = single
				}
			}
			accumulated += float64(difference) / 3
		}
	}
	result.meanError = accumulated / float64(result.total)
	return result
}

func channels(c color.Color) (int, int, int) {
	r, g, b, _ := c.RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8)
}

func absDifference(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// writeDiff 輸出差異遮罩：相同像素轉灰階並壓暗，不同像素塗紅，方便肉眼定位區塊。
func writeDiff(name string, reference, candidate image.Image) {
	bounds := reference.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r0, g0, b0 := channels(reference.At(x, y))
			r1, g1, b1 := channels(candidate.At(x, y))
			if r0 != r1 || g0 != g1 || b0 != b1 {
				out.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
				continue
			}
			grey := uint8((r0 + g0 + b0) / 3 / 3)
			out.Set(x, y, color.RGBA{R: grey, G: grey, B: grey, A: 255})
		}
	}
	output, err := os.Create(name)
	if err != nil {
		fail(fmt.Sprintf("create diff: %v", err))
	}
	if err := png.Encode(output, out); err != nil {
		_ = output.Close()
		fail(fmt.Sprintf("encode diff: %v", err))
	}
	if err := output.Close(); err != nil {
		fail(fmt.Sprintf("close diff: %v", err))
	}
}

// printDifferences 列出前 limit 個相異像素，供人工判讀差異形狀
// （整片錯色、單一通道偏移、或只有邊界像素不同）。
func printDifferences(reference, candidate image.Image, limit int) {
	bounds := reference.Bounds()
	printed := 0
	for y := bounds.Min.Y; y < bounds.Max.Y && printed < limit; y++ {
		for x := bounds.Min.X; x < bounds.Max.X && printed < limit; x++ {
			r0, g0, b0 := channels(reference.At(x, y))
			r1, g1, b1 := channels(candidate.At(x, y))
			if r0 == r1 && g0 == g1 && b0 == b1 {
				continue
			}
			fmt.Printf("pixel x=%d y=%d reference=%02X%02X%02X candidate=%02X%02X%02X delta=%d,%d,%d\n",
				x, y, r0, g0, b0, r1, g1, b1, r0-r1, g0-g1, b0-b1)
			printed++
		}
	}
}

func loadPNG(name string) image.Image {
	file, err := os.Open(name)
	if err != nil {
		fail(fmt.Sprintf("open %s: %v", name, err))
	}
	defer file.Close()
	decoded, err := png.Decode(file)
	if err != nil {
		fail(fmt.Sprintf("decode %s: %v", name, err))
	}
	return decoded
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "acan-imgdiff:", message)
	os.Exit(1)
}
