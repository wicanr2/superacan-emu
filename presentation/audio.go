package presentation

import (
	"encoding/binary"
	"fmt"
	"io"
)

// StereoResampler converts samples generated every sourceCycles clock ticks to
// a host PCM rate. Integer timestamps keep output deterministic across hosts;
// linear interpolation is presentation-only and never feeds machine state.
type StereoResampler struct {
	sourceClock   uint64
	sourceCycles  uint64
	outputRate    uint64
	emit          func(left, right int16)
	inputIndex    uint64
	nextOutput    uint64
	previousLeft  int16
	previousRight int16
	havePrevious  bool
}

func NewStereoResampler(sourceClock, sourceCycles, outputRate uint64, emit func(int16, int16)) *StereoResampler {
	if sourceClock == 0 || sourceCycles == 0 || outputRate == 0 || emit == nil {
		panic("presentation: invalid stereo resampler configuration")
	}
	return &StereoResampler{sourceClock: sourceClock, sourceCycles: sourceCycles, outputRate: outputRate, emit: emit}
}

func (r *StereoResampler) Push(left, right int16) {
	index := r.inputIndex
	inputTime := index * r.sourceCycles * r.outputRate
	for r.nextOutput*r.sourceClock <= inputTime {
		outputLeft, outputRight := left, right
		if r.havePrevious {
			start := (index - 1) * r.sourceCycles * r.outputRate
			numerator := r.nextOutput*r.sourceClock - start
			denominator := r.sourceCycles * r.outputRate
			outputLeft = interpolate16(r.previousLeft, left, numerator, denominator)
			outputRight = interpolate16(r.previousRight, right, numerator, denominator)
		}
		r.emit(outputLeft, outputRight)
		r.nextOutput++
	}
	r.previousLeft, r.previousRight = left, right
	r.havePrevious = true
	r.inputIndex++
}

func interpolate16(from, to int16, numerator, denominator uint64) int16 {
	left := int64(from) * int64(denominator-numerator)
	right := int64(to) * int64(numerator)
	return int16((left + right) / int64(denominator))
}

// EncodePCM16WAV writes little-endian signed 16-bit stereo PCM with a canonical
// 44-byte RIFF/WAVE header.
func EncodePCM16WAV(output io.Writer, sampleRate uint32, pcm []byte) error {
	if sampleRate == 0 {
		return fmt.Errorf("presentation: WAV sample rate must be nonzero")
	}
	if len(pcm)%4 != 0 {
		return fmt.Errorf("presentation: stereo PCM byte count %d is not frame-aligned", len(pcm))
	}
	if uint64(len(pcm)) > uint64(^uint32(0))-36 {
		return fmt.Errorf("presentation: PCM is too large for RIFF/WAVE")
	}
	var header [44]byte
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+len(pcm)))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 2)
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], sampleRate*4)
	binary.LittleEndian.PutUint16(header[32:34], 4)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(len(pcm)))
	if _, err := output.Write(header[:]); err != nil {
		return err
	}
	_, err := output.Write(pcm)
	return err
}
