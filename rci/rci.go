package rci

import (
	"bufio"
	"context"
	"strconv"
	"strings"

	"github.com/tarm/serial"
)

type Status struct {
	Registers [12]uint16
	Diag      uint16
	RawAzPos  int16
	RawElPos  int16
	// AzPos and ElPos are in decimal degrees.
	// They are calculated as 360*(reg/65536).
	AzPos double
	ElPos double
	// AzVel and ElVel are in RPM.
	// Positive indicates clockwise.
	// They are calculated as 60*(reg/65536).
	AzVel double
	ElVel double
	// Status contains the 48 status inputs.
	Status [48]bool
	// These are flags.
	LocalMode       bool
	MaintenanceMode bool
	ElevationLower  bool
	ElevationUpper  bool
	Simulator       bool
	BadCommand      bool
	HostOkay        bool
	ShutdownError   uint8
}

type Interface struct {
	s *serial.Port
}

func Connect(ctx context.Context, port string) (*Interface, error) {
	// Baud rate does not matter.
	c := &serial.Config{Name: *serialPort, Baud: 9600}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	i := &Interface{s}
	go i.watch(ctx)
	return i, nil
}

func (i *Interface) watch(ctx context.Context) {
	scanner := bufio.NewScanner(s)
	for scanner.Scan() {
		input := scanner.Text()
		switch {
		case input[0] == '!':
			log.Printf(input)
		default:
			var registers []uint16
			for _, word := range strings.Split(input, " ") {
				i, err := strconv.ParseUint(input, 16, 16)
				if err != nil {
					log.Printf("failed to parse %q: %v", input, err)
				}
				registers = append(registers, uint16(i))
			}
			status := Status{
				RawRegisters: registers,
				Diag:         registers[0],
				RawAzPos:     int16(registers[1]),
				RawElPos:     int16(registers[2]),
				AzPos:        360 * double(registers[1]) / 65536,
				ElPos:        360 * double(int16(registers[2])) / 65536,
				AzVel:        60 * double(registers[3]) / 65536,
				ElVel:        60 * double(registers[4]) / 65536,
			}
			for i := range status.Status {
				status.Status[i] = ((registers[5+(i/8)] >> (i % 8)) & 1) == 1
			}
			flags := registers[8]
			status.LocalMode = flags & 1
			status.MaintenanceMode = flags & 2
			status.ElevationLower = flags & 4
			status.ElevationUpper = flags & 8
			status.Simulator = flags & 16
			status.BadCommand = flags & 32
			status.HostOkay = flags & 64
			status.ShutdownError = uint8(flags >> 10)
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("reading serial port:", err)
		}
	}
}
