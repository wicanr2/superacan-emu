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
	"strconv"
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
	referenceOut := flag.String("reference-out", "", "write the reference after --reference-unstretch, for side-by-side figures")
	unstretch := flag.String("reference-unstretch", "", "undo the oracle's nearest-neighbour upscale: N for width only, or WxH+X+Y to undo both axes and place the native picture at (X,Y); the geometry also becomes the compared region")
	flag.Parse()

	if *reference == "" || (*candidate == "" && *candidateDir == "") {
		fail("--reference and one of --candidate/--candidate-dir are required")
	}

	referenceImage := loadPNG(*reference)
	region := referenceImage.Bounds()
	if *unstretch != "" {
		native, placement := parseUnstretch(*unstretch, referenceImage.Bounds())
		referenceImage = unstretchNearest(referenceImage, native, placement)
		region = placement
	}
	if *activeWidth > 0 && region.Min.X+*activeWidth < region.Max.X {
		region.Max.X = region.Min.X + *activeWidth
	}
	if *referenceOut != "" {
		writePNG(*referenceOut, referenceImage)
	}
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
		results = append(results, compare(referenceImage, loadPNG(name), name, region))
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
		printDifferences(referenceImage, loadPNG(results[0].name), *listDifferences, region)
	}
	if *diffOut != "" {
		writeDiff(*diffOut, referenceImage, loadPNG(results[0].name), region)
	}
}

// compare 逐像素比對 RGB 三通道；alpha 不比較，因為兩端來源的 alpha 語意不同。
// region 是有意義的比較範圍：oracle 截圖固定 320×240，實際顯示區可能更窄或更矮，
// 範圍外的像素在兩邊沒有相同語意，不應計入差異。
func compare(reference, candidate image.Image, name string, region image.Rectangle) comparison {
	if candidate.Bounds() != reference.Bounds() {
		fail(fmt.Sprintf("%s has bounds %v, reference has %v", name, candidate.Bounds(), reference.Bounds()))
	}
	result := comparison{name: name, total: region.Dx() * region.Dy()}
	var accumulated float64
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
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
func writeDiff(name string, reference, candidate image.Image, region image.Rectangle) {
	out := image.NewRGBA(reference.Bounds())
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
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
	writePNG(name, out)
}

func writePNG(name string, img image.Image) {
	output, err := os.Create(name)
	if err != nil {
		fail(fmt.Sprintf("create %s: %v", name, err))
	}
	if err := png.Encode(output, img); err != nil {
		_ = output.Close()
		fail(fmt.Sprintf("encode %s: %v", name, err))
	}
	if err := output.Close(); err != nil {
		fail(fmt.Sprintf("close %s: %v", name, err))
	}
}

// printDifferences 列出前 limit 個相異像素，供人工判讀差異形狀
// （整片錯色、單一通道偏移、或只有邊界像素不同）。
func printDifferences(reference, candidate image.Image, limit int, region image.Rectangle) {
	printed := 0
	for y := region.Min.Y; y < region.Max.Y && printed < limit; y++ {
		for x := region.Min.X; x < region.Max.X && printed < limit; x++ {
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

// parseUnstretch 解析 --reference-unstretch。接受兩種寫法：只給寬度的 "256"
// （高度不動、原地擺放），或完整幾何 "256x224+0+8"。回傳原生尺寸與擺放位置，
// 擺放位置同時就是有意義的比較範圍。
func parseUnstretch(spec string, bounds image.Rectangle) (image.Point, image.Rectangle) {
	if width, err := strconv.Atoi(spec); err == nil {
		return image.Pt(width, bounds.Dy()), image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+width, bounds.Max.Y)
	}
	var w, h, x, y int
	if _, err := fmt.Sscanf(spec, "%dx%d+%d+%d", &w, &h, &x, &y); err != nil {
		fail(fmt.Sprintf("--reference-unstretch %q: 需要 N 或 WxH+X+Y", spec))
	}
	if w <= 0 || h <= 0 {
		fail(fmt.Sprintf("--reference-unstretch %q: 尺寸要大於零", spec))
	}
	return image.Pt(w, h), image.Rect(bounds.Min.X+x, bounds.Min.Y+y, bounds.Min.X+x+w, bounds.Min.Y+y+h)
}

// unstretchNearest 還原 oracle 的最近鄰放大。Bcan 0.0.8b 的截圖孔徑固定 320×240，
// 但 UM6618 實際輸出的顯示區更小（256 模式是 256 欄；量到的行數是 224），Bcan 以
// dst = floor(src * 孔徑 / 原生) 直接複製整欄或整列填滿，因此每 5 欄有一對相同、
// 每 15 列有一對相同。還原取 src = ceil(dst * 孔徑 / 原生)，即每組丟掉重複的那一份，
// 再擺到 placement 指定的位置；範圍外補黑。
func unstretchNearest(source image.Image, native image.Point, placement image.Rectangle) image.Image {
	bounds := source.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, color.RGBA{A: 255})
		}
	}
	for targetY := range native.Y {
		sourceY := bounds.Min.Y + scaleBack(targetY, bounds.Dy(), native.Y)
		sourceY = min(sourceY, bounds.Max.Y-1)
		for targetX := range native.X {
			sourceX := bounds.Min.X + scaleBack(targetX, bounds.Dx(), native.X)
			sourceX = min(sourceX, bounds.Max.X-1)
			destX, destY := placement.Min.X+targetX, placement.Min.Y+targetY
			if destX < bounds.Min.X || destX >= bounds.Max.X || destY < bounds.Min.Y || destY >= bounds.Max.Y {
				continue
			}
			r, g, b := channels(source.At(sourceX, sourceY))
			out.Set(destX, destY, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
		}
	}
	return out
}

// scaleBack 是 dst = floor(src * aperture / native) 的反函數：取 ceil，
// 也就是每一組重複裡的最後一份。
func scaleBack(target, aperture, native int) int {
	return (target*aperture + native - 1) / native
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
