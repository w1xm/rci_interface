package modbus

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/goburrow/modbus"
	"github.com/w1xm/rci_interface/sequencer/modbushttp"
)

type modbusHandler interface {
	modbus.ClientHandler
	Connect() error
	Close() error
}

type Client struct {
	// Port and BaudRate create a local serial connection
	Port string
	// BaudRate defaults to 19200
	BaudRate int
	SlaveId  byte
	// URL creates a remote connection
	URL string

	// Poll function to be called in a loop while the connection is active
	Poll func() error

	handler modbusHandler
	modbus.Client
}

func (c *Client) Connect(ctx context.Context) error {
	if c.URL != "" {
		c.handler = modbushttp.NewClient(c.URL)
	} else {
		handler := modbus.NewRTUClientHandler(c.Port)
		handler.BaudRate = c.BaudRate
		handler.DataBits = 8
		handler.Parity = "N"
		handler.StopBits = 1
		handler.Timeout = 1 * time.Second
		handler.SlaveId = c.SlaveId
		c.handler = handler
	}

	_ = os.Stderr
	//handler.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	c.Client = modbus.NewClient(c.handler)
	go c.reconnectLoop(ctx)
	return nil
}

func (c *Client) reconnectLoop(ctx context.Context) {
	port := c.URL
	if port == "" {
		port = c.Port
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}

		err := c.handler.Connect()
		if err != nil {
			log.Printf("opening %q: %v", port, err)
			continue
		}
		if err := c.watch(ctx); err != nil {
			log.Printf("watching %q: %v", port, err)
		}
	}
}

func (c *Client) watch(ctx context.Context) error {
	defer c.handler.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := c.Poll(); err != nil {
			return err
		}
	}
}

func (c *Client) WriteCoil(coil int, value bool) error {
	var v uint16
	if value {
		v = 0xFF00
	}
	_, err := c.WriteSingleCoil(uint16(coil), v)
	return err
}

func BytesToBits(bs []byte) []bool {
	var out []bool
	for _, b := range bs {
		for i := 0; i < 8; i++ {
			out = append(out, (b>>uint(i)&1) == 1)
		}
	}
	return out
}
