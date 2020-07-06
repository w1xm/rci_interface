package sequencer

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/w1xm/rci_interface/internal/modbus"
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
	statusCallback StatusCallback
	mu             sync.Mutex
	client         *modbus.Client
	bands          int
	coils          []bool
	inputs         []bool
}

func Connect(ctx context.Context, port string, baud int, statusCallback StatusCallback) (*Sequencer, error) {
	s := &Sequencer{
		client: &modbus.Client{
			Port:     port,
			BaudRate: baud,
			SlaveId:  1,
		},
		statusCallback: statusCallback,
	}
	s.client.Poll = s.pollOnce
	return s, s.client.Connect(ctx)
}

func ConnectRemote(ctx context.Context, url string, statusCallback StatusCallback) (*Sequencer, error) {
	s := &Sequencer{
		client: &modbus.Client{
			URL: url,
		},
		statusCallback: statusCallback,
	}
	s.client.Poll = s.pollOnce
	return s, s.client.Connect(ctx)
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
	s.coils = modbus.BytesToBits(coils)
	s.inputs = modbus.BytesToBits(inputs)
	s.notifyStatus()
	return nil
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

func (s *Sequencer) SetBandTX(band int, tx bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if band >= s.bands {
		return fmt.Errorf("invalid band %d", band)
	}
	return s.client.WriteCoil(band, tx)
}
func (s *Sequencer) SetBandRX(band int, rx bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if band >= s.bands {
		return fmt.Errorf("invalid band %d", band)
	}
	return s.client.WriteCoil(s.bands+band, rx)
}
