package cps20

import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/w1xm/rci_interface/internal/modbus"
)

type Status struct {
	CommandSpinupDelay int

	CommandAzEnabled bool
	CommandElEnabled bool

	AmplidynesActive bool
	AzActive         bool
	ElActive         bool
}

type StatusCallback func(status Status)

type CPS20 struct {
	statusCallback StatusCallback
	mu             sync.Mutex
	client         *modbus.Client
	relays         int
	delay          int
	coils          []bool
	inputs         []bool
}

func Connect(ctx context.Context, port string, baud int, statusCallback StatusCallback) (*CPS20, error) {
	c := &CPS20{
		client: &modbus.Client{
			Port:     port,
			BaudRate: baud,
			SlaveId:  1,
		},
		statusCallback: statusCallback,
	}
	c.client.Poll = c.pollOnce
	return c, c.client.Connect(ctx)
}

func (c *CPS20) pollOnce() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	results, err := c.client.ReadInputRegisters(0, 1)
	if err != nil {
		return err
	}
	relays := binary.BigEndian.Uint16(results)

	results, err = c.client.ReadHoldingRegisters(0, 1)
	if err != nil {
		return err
	}
	c.delay = int(binary.BigEndian.Uint16(results))

	coils, err := c.client.ReadCoils(0, relays)
	if err != nil {
		return err
	}
	inputs, err := c.client.ReadDiscreteInputs(0, relays+1)
	if err != nil {
		return err
	}
	c.relays = int(relays)
	c.coils = modbus.BytesToBits(coils)
	c.inputs = modbus.BytesToBits(inputs)
	c.notifyStatus()
	return nil
}

func (c *CPS20) notifyStatus() {
	status := c.parseRegisters()
	c.statusCallback(status)
}

func (c *CPS20) parseRegisters() Status {
	status := Status{
		CommandSpinupDelay: c.delay,

		CommandAzEnabled: c.coils[0],
		CommandElEnabled: c.coils[1],

		AmplidynesActive: c.inputs[0],
		AzActive:         c.inputs[1],
		ElActive:         c.inputs[2],
	}
	return status
}

func (c *CPS20) SetAmplidynesEnabled(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.client.WriteCoil(0, enabled); err != nil {
		return err
	}
	if err := c.client.WriteCoil(1, enabled); err != nil {
		return err
	}
	return nil
}
