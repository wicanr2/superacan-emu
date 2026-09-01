package presentation

import (
	"encoding/binary"
	"sync"
)

// PCM16StereoStream is a bounded, concurrency-safe PCM queue for a host audio
// callback. Read always fills the requested buffer and supplies silence during
// underruns, so presentation timing can never stall or feed back into the
// emulated machine.
type PCM16StereoStream struct {
	mu       sync.Mutex
	buffer   []byte
	capacity int
}

func NewPCM16StereoStream(capacityFrames int) *PCM16StereoStream {
	if capacityFrames <= 0 {
		panic("presentation: PCM stream capacity must be positive")
	}
	return &PCM16StereoStream{capacity: capacityFrames * 4}
}

func (s *PCM16StereoStream) Push(left, right int16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buffer)+4 > s.capacity {
		copy(s.buffer, s.buffer[4:])
		s.buffer = s.buffer[:len(s.buffer)-4]
	}
	var frame [4]byte
	binary.LittleEndian.PutUint16(frame[0:2], uint16(left))
	binary.LittleEndian.PutUint16(frame[2:4], uint16(right))
	s.buffer = append(s.buffer, frame[:]...)
}

func (s *PCM16StereoStream) Read(output []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := copy(output, s.buffer)
	clear(output[copied:])
	s.buffer = s.buffer[copied:]
	if len(s.buffer) == 0 {
		s.buffer = s.buffer[:0]
	}
	return len(output), nil
}

func (s *PCM16StereoStream) BufferedFrames() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buffer) / 4
}
