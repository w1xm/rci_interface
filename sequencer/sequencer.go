package sequencer

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/goburrow/modbus"
)

type Band struct {
	CommandTX bool
	CommandRX bool
	TX        bool
}

type Status struct {
	Error bool
	Bands []Band
}

type StatusCallback func(status Status)

type Sequencer struct {
	handler        *modbus.RTUClientHandler
	statusCallback StatusCallback
	mu             sync.Mutex
	client         modbus.Client
	bands          int
	coils          []bool
	inputs         []bool
}

func Connect(ctx context.Context, port string, baud int, statusCallback StatusCallback) (*Sequencer, error) {
	handler := modbus.NewRTUClientHandler(port)
	handler.BaudRate = baud
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	handler.Timeout = 1 * time.Second
	handler.SlaveId = 1
	handler.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	client := modbus.NewClient(handler)
	s := &Sequencer{handler: handler, client: client, statusCallback: statusCallback}
	go s.reconnectLoop(ctx, port)
	return s, nil
}

func (s *Sequencer) reconnectLoop(ctx context.Context, port string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}

		err := s.handler.Connect()
		if err != nil {
			log.Printf("opening %q: %v", port, err)
			continue
		}
		s.watch(ctx)
	}
}

func (s *Sequencer) watch(ctx context.Context) error {
	defer s.handler.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := s.pollOnce(); err != nil {
			return err
		}
	}
}

func (s *Sequencer) pollOnce() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	results, err := s.client.ReadInputRegisters(0, 1)
	if err != nil {
		return err
	}
	bands := binary.BigEndian.Uint16(results)
	coils, err := s.client.ReadCoils(0, bands*2)
	if err != nil {
		return err
	}
	inputs, err := s.client.ReadDiscreteInputs(0, bands+1)
	if err != nil {
		return err
	}
	s.bands = int(bands)
	s.coils = bytesToBits(coils)
	s.inputs = bytesToBits(inputs)
	s.notifyStatus()
	return nil
}

func bytesToBits(bs []byte) []bool {
	var out []bool
	for _, b := range bs {
		for i := 0; i < 8; i++ {
			out = append(out, (b>>uint(i)&1) == 1)
		}
	}
	return out
}

func (s *Sequencer) notifyStatus() {
	status := s.parseRegisters()
	s.statusCallback(status)
}

func (s *Sequencer) parseRegisters() Status {
	status := Status{
		Error: s.inputs[0],
	}
	for i := 0; i < s.bands; i++ {
		status.Bands = append(status.Bands, Band{
			CommandTX: s.coils[i],
			TX:        s.inputs[i+1],
			CommandRX: s.coils[s.bands+i],
		})
	}
	return status
}

func (s *Sequencer) writeCoil(coil int, value bool) error {
	var v uint16
	if value {
		v = 0xFF00
	}
	_, err := s.client.WriteSingleCoil(uint16(coil), v)
	return err
}

func (s *Sequencer) SetBandTX(band int, tx bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if band >= s.bands {
		return fmt.Errorf("invalid band %d", band)
	}
	return s.writeCoil(band, tx)
}
func (s *Sequencer) SetBandRX(band int, rx bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if band >= s.bands {
		return fmt.Errorf("invalid band %d", band)
	}
	return s.writeCoil(s.bands+band, rx)
}
