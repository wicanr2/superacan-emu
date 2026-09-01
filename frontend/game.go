package frontend

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/presentation"
)

const DefaultFrameInstructionBound uint64 = 2_000_000

var playerOneKeys = [...]struct {
	key    ebiten.Key
	button uint16
}{
	{ebiten.KeyZ, machine.ButtonA}, {ebiten.KeyX, machine.ButtonB},
	{ebiten.KeyEnter, machine.ButtonStart}, {ebiten.KeyShiftRight, machine.ButtonSelect},
	{ebiten.KeyArrowUp, machine.ButtonUp}, {ebiten.KeyArrowDown, machine.ButtonDown},
	{ebiten.KeyArrowLeft, machine.ButtonLeft}, {ebiten.KeyArrowRight, machine.ButtonRight},
	{ebiten.KeyA, machine.ButtonX}, {ebiten.KeyS, machine.ButtonY},
	{ebiten.KeyQ, machine.ButtonL}, {ebiten.KeyW, machine.ButtonR},
}

// Game is a thin Ebitengine presentation adapter over the shared machine core.
type Game struct {
	System           *machine.System
	InstructionBound uint64
	MaxFrames        uint64
	CompletedFrames  uint64
	frame            *ebiten.Image
	pixels           []byte
}

func NewGame(system *machine.System) *Game {
	return &Game{
		System: system, InstructionBound: DefaultFrameInstructionBound,
		frame: ebiten.NewImage(umc6618.Width, umc6618.Height),
	}
}

func (g *Game) Update() error {
	var pressed uint16
	for _, binding := range playerOneKeys {
		if ebiten.IsKeyPressed(binding.key) {
			pressed |= binding.button
		}
	}
	g.System.SoundBus.SetPad(0, machine.PadState(pressed))
	if _, err := g.System.RunFrame(g.InstructionBound); err != nil {
		return err
	}
	g.pixels = presentation.ARGBToRGBA(g.pixels, g.System.Bus.Video().Framebuffer())
	g.CompletedFrames++
	if g.MaxFrames != 0 && g.CompletedFrames >= g.MaxFrames {
		return ebiten.Termination
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.frame.WritePixels(g.pixels)
	screen.DrawImage(g.frame, nil)
}

func (g *Game) Layout(_, _ int) (int, int) { return umc6618.Width, umc6618.Height }
