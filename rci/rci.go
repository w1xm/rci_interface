package rci

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/tarm/serial"
)

type Status struct {
	RawRegisters [12]uint16
	Diag         uint16
	RawAzPos     int16
	RawElPos     int16
	// AzPos and ElPos are in decimal degrees.
	// They are calculated as 360*(reg/65536).
	AzPos float64
	ElPos float64
	// AzVel and ElVel are in RPM.
	// Positive indicates clockwise.
	// They are calculated as 60*(reg/65536).
	AzVel float64
	ElVel float64
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

func ParseRegisters(registers []uint16) Status {
	var arr [12]uint16
	for i, v := range registers {
		if i >= len(arr) {
			continue
		}
		arr[i] = v
	}
	status := Status{
		RawRegisters: [12]uint16(arr),
		Diag:         registers[0],
		RawAzPos:     int16(registers[1]),
		RawElPos:     int16(registers[2]),
		AzPos:        360 * float64(registers[1]) / 65536,
		ElPos:        360 * float64(int16(registers[2])) / 65536,
		AzVel:        60 * float64(registers[3]) / 65536,
		ElVel:        60 * float64(registers[4]) / 65536,
	}
	for i := range status.Status {
		status.Status[i] = ((registers[5+(i/8)] >> (uint(i) % 8)) & 1) == 1
	}
	flags := registers[8]
	status.LocalMode = flags&1 != 0
	status.MaintenanceMode = flags&2 != 0
	status.ElevationLower = flags&4 != 0
	status.ElevationUpper = flags&8 != 0
	status.Simulator = flags&16 != 0
	status.BadCommand = flags&32 != 0
	status.HostOkay = flags&64 != 0
	status.ShutdownError = uint8(flags >> 10)
	return status
}

type StatusCallback func(status Status)

type RCI struct {
	s              *serial.Port
	statusCallback StatusCallback
	mu             sync.Mutex
	lastDiag       uint16
}

func Connect(ctx context.Context, port string, statusCallback StatusCallback) (*RCI, error) {
	// Baud rate does not matter.
	c := &serial.Config{Name: port, Baud: 9600}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	r := &RCI{s: s, statusCallback: statusCallback}
	go r.watch(ctx)
	return r, nil
}

func (r *RCI) watch(ctx context.Context) {
	// TODO: Close when ctx is canceled.
	defer r.s.Close()
	scanner := bufio.NewScanner(r.s)
	for scanner.Scan() {
		input := scanner.Text()
		switch {
		case input[0] == '!':
			log.Printf(input)
		case len(input) > 0:
			var registers []uint16
			for _, word := range strings.Split(input, " ") {
				i, err := strconv.ParseUint(word, 16, 16)
				if err != nil {
					log.Printf("failed to parse %q: %v", input, err)
				}
				registers = append(registers, uint16(i))
			}
			status := ParseRegisters(registers)
			r.statusCallback(status)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("reading serial port:", err)
		}
	}
}

func (r *RCI) Write(register int, values ...uint16) {
	out := []string{fmt.Sprintf("%x", register)}
	for _, v := range values {
		out = append(out, fmt.Sprintf("%x", v))
	}
	outStr := "w" + strings.Join(out, " ")
	log.Printf("Writing: %s", outStr)
	if _, err := r.s.Write([]byte(outStr)); err != nil {
		log.Print(err)
	}
}

const (
	SERVO_NONE     uint16 = 0
	SERVO_POSITION uint16 = 1
	SERVO_VELOCITY uint16 = 2
)

func (r *RCI) Stop() {
	r.lastDiag++
	r.Write(0, r.lastDiag)
	r.Write(3, SERVO_NONE)
	r.Write(6, SERVO_NONE)
}

func (r *RCI) SetAzimuthPosition(angle float64) {
	r.lastDiag++
	r.Write(0, r.lastDiag)
	r.Write(1, uint16(angle/360*65536))
	r.Write(3, SERVO_POSITION)
}

func (r *RCI) SetElevationPosition(angle float64) {
	r.lastDiag++
	r.Write(0, r.lastDiag)
	r.Write(4, uint16(angle/360*65536))
	r.Write(6, SERVO_POSITION)
}
