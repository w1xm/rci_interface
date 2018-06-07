package rci

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

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
	// AzVel and ElVel are in degrees/second.
	// Positive indicates clockwise.
	// They are calculated as 360*(reg/65536).
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

	WriteRegisters                 [11]uint16
	CommandDiag                    uint16
	CommandAzPos, CommandElPos     float64
	CommandAzVel, CommandElVel     float64
	CommandAzFlags, CommandElFlags string
	CommandStatus                  [48]bool
}

func regToSigned(reg uint16) float64 {
	return 360 * float64(int16(reg)) / 65536
}

func regToFlags(reg uint16) string {
	switch reg {
	case 0:
		return "NONE"
	case 1:
		return "POSITION"
	case 2:
		return "VELOCITY"
	}
	return "UNKNOWN"
}

func (r *RCI) parseRegisters() Status {
	registers := r.readRegisters
	status := Status{
		RawRegisters: registers,
		Diag:         registers[0],
		RawAzPos:     int16(registers[1]),
		RawElPos:     int16(registers[2]),
		AzPos:        360 * float64(registers[1]) / 65536,
		ElPos:        regToSigned(registers[2]),
		AzVel:        regToSigned(registers[3]),
		ElVel:        regToSigned(registers[4]),

		WriteRegisters: r.writeRegisters,
		CommandDiag:    r.writeRegisters[0],
		CommandAzPos:   360 * float64(r.writeRegisters[1]) / 65536,
		CommandAzVel:   regToSigned(r.writeRegisters[2]),
		CommandElPos:   360 * float64(r.writeRegisters[4]) / 65536,
		CommandElVel:   regToSigned(r.writeRegisters[5]),
		CommandAzFlags: regToFlags(r.writeRegisters[3]),
		CommandElFlags: regToFlags(r.writeRegisters[6]),
	}
	for i := range status.Status {
		status.Status[i] = ((registers[5+(i/16)] >> (uint(i) % 16)) & 1) == 1
	}
	for i := range status.CommandStatus {
		status.CommandStatus[i] = ((r.writeRegisters[5+(i/8)] >> (uint(i) % 8)) & 1) == 1
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
	readRegisters  [12]uint16
	writeRegisters [11]uint16
	lastDiag       uint16
}

func Connect(ctx context.Context, port string, statusCallback StatusCallback) (*RCI, error) {
	r := &RCI{statusCallback: statusCallback}
	go r.reconnectLoop(ctx, port)
	return r, nil
}

func (r *RCI) reconnectLoop(ctx context.Context, port string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
		// Baud rate does not matter.
		c := &serial.Config{Name: port, Baud: 9600}
		s, err := serial.OpenPort(c)
		if err != nil {
			log.Printf("opening %q: %v", port, err)
			continue
		}
		log.Printf("opened %q", port)
		r.mu.Lock()
		r.s = s
		r.mu.Unlock()
		r.watch(ctx)
		r.mu.Lock()
		r.s = nil
		r.mu.Unlock()
	}
}

func (r *RCI) watch(ctx context.Context) {
	// TODO: Close when ctx is canceled.
	defer r.s.Close()
	scanner := bufio.NewScanner(r.s)
	for scanner.Scan() {
		input := scanner.Text()
		if len(input) < 1 {
			continue
		}
		switch {
		case input[0] == '!':
			log.Printf(input)
		case input[0] == 'r':
			r.mu.Lock()
			for i, word := range strings.Split(input[1:len(input)-1], " ") {
				v, err := strconv.ParseUint(word, 16, 16)
				if err != nil {
					log.Printf("failed to parse %q: %v", input, err)
				}
				r.readRegisters[i] = uint16(v)
			}
			r.notifyStatus()
			r.mu.Unlock()
		default:
			log.Printf("unknown input: %s", input)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("reading serial port: %s", err)
	}
}

func (r *RCI) notifyStatus() {
	status := r.parseRegisters()
	r.statusCallback(status)
}

func (r *RCI) Write(register int, values ...uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.s == nil {
		return
	}
	out := []string{fmt.Sprintf("%x", register)}
	for i, v := range values {
		r.writeRegisters[register+i] = v
		out = append(out, fmt.Sprintf("%x", v))
	}
	outStr := "w" + strings.Join(out, " ") + "\n"
	log.Printf("Writing: %s", outStr)
	if _, err := r.s.Write([]byte(outStr)); err != nil {
		log.Print(err)
	}
	r.notifyStatus()
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

func (r *RCI) SetAzimuthVelocity(angle float64) {
	r.lastDiag++
	r.Write(0, r.lastDiag)
	r.Write(2, uint16(angle/360*65536))
	r.Write(3, SERVO_VELOCITY)
}

func (r *RCI) SetElevationVelocity(angle float64) {
	r.lastDiag++
	r.Write(0, r.lastDiag)
	r.Write(5, uint16(angle/360*65536))
	r.Write(6, SERVO_VELOCITY)
}

func (r *RCI) ExitShutdown() {
	r.Stop()
	// Toggling this bit from 0 to 1 to 0 in a time not less than
	// 0.1 seconds, but not greater than 1.0 second, will force
	// the RCI to exit from any prior shutdown condition. The
	// toggling feature prevents the bit from accidentally being
	// left active, since doing so would prevent genuine shutdowns
	// from proceeding normally.
	r.Write(10, 1)
	time.Sleep(200 * time.Millisecond)
	r.Write(10, 0)
}
