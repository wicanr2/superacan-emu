package ui_test

import (
	"os/exec"
	"strings"
	"testing"
)

// ui 不得相依任何前端套件。這條與「模擬核心不 import Ebitengine」同層級：
// 介面一旦碰到前端型別，換平台就得改介面本身。
func TestNoFrontendDependencies(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("環境沒有 go 工具鏈")
	}
	output, err := exec.Command("go", "list", "-deps", "github.com/wicanr2/superacan-emu/ui").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, output)
	}
	banned := []string{
		"github.com/hajimehoshi/ebiten",
		"github.com/jezek/xgb",
		"github.com/ebitengine/gomobile",
		"github.com/ebitengine/oto",
		"github.com/wicanr2/superacan-emu/frontend",
		"github.com/wicanr2/superacan-emu/machine",
	}
	for _, line := range strings.Split(string(output), "\n") {
		for _, bad := range banned {
			if strings.HasPrefix(strings.TrimSpace(line), bad) {
				t.Errorf("ui 相依了 %s", line)
			}
		}
	}
}
