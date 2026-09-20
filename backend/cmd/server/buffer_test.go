package main

import (
	"bytes"
	"sync"
)

// syncBuffer collects log output written from the server's goroutines while the test reads
// it from its own.
type syncBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.buffer.Write(p)
}

func (s *syncBuffer) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.buffer.String()
}
