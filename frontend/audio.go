package frontend

import (
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/wicanr2/superacan-emu/chip/umc6619"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/presentation"
)

const (
	AudioSampleRate       = 48_000
	audioBufferTimeMillis = 200
)

// Audio presents deterministic UMC6619 samples to Ebitengine. Its buffering
// and interpolation are host-only and never affect emulated timing or state.
type Audio struct {
	device *umc6619.Device
	stream *presentation.PCM16StereoStream
	player *audio.Player
}

func NewAudio(system *machine.System) (*Audio, error) {
	stream := presentation.NewPCM16StereoStream(AudioSampleRate * audioBufferTimeMillis / 1000)
	resampler := presentation.NewStereoResampler(
		umc6619.ClockHz, umc6619.CyclesPerSample, AudioSampleRate, stream.Push,
	)
	device := system.SoundBus.Audio()
	device.SetSampleSink(func(sample umc6619.Sample) { resampler.Push(sample.Left, sample.Right) })
	player, err := audio.NewPlayer(audio.NewContext(AudioSampleRate), stream)
	if err != nil {
		device.SetSampleSink(nil)
		return nil, err
	}
	player.Play()
	return &Audio{device: device, stream: stream, player: player}, nil
}

func (a *Audio) Close() error {
	a.device.SetSampleSink(nil)
	return a.player.Close()
}

func (a *Audio) BufferedFrames() int { return a.stream.BufferedFrames() }
