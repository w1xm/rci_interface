package terminal

import (
	"bufio"
	"context"
	"io"
	"log"
	"sync"
	"time"

	"github.com/jaguilar/vt100"
	"github.com/tarm/serial"
)

type Terminal struct {
	mu       sync.RWMutex
	cond     *sync.Cond
	s        *serial.Port
	vt       *vt100.VT100
	frameNum int
}

func Open(ctx context.Context, port string, baud int) *Terminal {
	t := &Terminal{vt: vt100.NewVT100(80, 24)}
	t.cond = sync.NewCond(t.mu.RLocker())
	go t.reconnectLoop(ctx, port, baud)
	return t
}

func (t *Terminal) reconnectLoop(ctx context.Context, port string, baud int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
		c := &serial.Config{Name: port, Baud: baud}
		s, err := serial.OpenPort(c)
		if err != nil {
			log.Printf("opening %q: %v", port, err)
			continue
		}
		log.Printf("opened %q", port)
		t.mu.Lock()
		t.s = s
		t.mu.Unlock()
		t.process(ctx)
		t.mu.Lock()
		t.s = nil
		t.mu.Unlock()
	}
}

func (t *Terminal) process(ctx context.Context) {
	defer t.s.Close()
	br := bufio.NewReader(t.s)
	for {
		cmd, err := vt100.Decode(br)
		switch err {
		case io.EOF:
			return
		case nil:
		default:
			log.Printf("reading serial port: %s", err)
			return
		}
		t.mu.Lock()
		err = t.vt.Process(cmd)
		t.frameNum++
		t.cond.Broadcast()
		t.mu.Unlock()
		switch err.(type) {
		case vt100.UnsupportedError:
		default:
			log.Printf("processing escape sequence: %s", err)
		}
	}
}

func (t *Terminal) WriteString(input string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, err := t.s.Write([]byte(input))
	return err
}

type FrameWatcher struct {
	t        *Terminal
	frameNum int
}

func (t *Terminal) WatchFrames() *FrameWatcher {
	return &FrameWatcher{t: t}
}

// Next waits for a new frame, then returns its HTML.
func (fw *FrameWatcher) Next() string {
	fw.t.mu.RLock()
	defer fw.t.mu.RUnlock()
	for {
		fw.t.cond.Wait()
		if fw.t.frameNum > fw.frameNum {
			fw.frameNum = fw.t.frameNum
			return fw.t.vt.HTML()
		}
	}
}
