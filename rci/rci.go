package rci

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tarm/serial"
	"github.com/w1xm/rci_interface/rotator"
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
	// Moving indicates whether there is a pending move that has not yet completed.
	Moving bool
	// MovingDisabled indicates that move commands are current disabled (e.g. because amplidynes are not running).
	MovingDisabled bool

	WriteRegisters                 [11]uint16
	CommandDiag                    uint16
	CommandAzPos, CommandElPos     float64
	CommandAzVel, CommandElVel     float64
	CommandAzFlags, CommandElFlags string
	CommandStatus                  [48]bool
}

func (s Status) Clone() rotator.Status {
	return s
}

func (s Status) AzimuthPosition() float64 {
	return s.AzPos
}

func (s Status) ElevationPosition() float64 {
	return s.ElPos
}

func (s Status) AzElVelocity() (float64, float64) {
	return s.AzVel, s.ElVel
}

func (s Status) AzimuthCommand() (string, float64) {
	return s.CommandAzFlags, s.CommandAzPos
}

func (s Status) ElevationCommand() (string, float64) {
	return s.CommandElFlags, s.CommandElPos
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

const (
	QUIESCENT_VELOCITY = 0.2
	QUIESCENT_TIME     = 1 * time.Second
)

func (r *RCI) parseRegisters() Status {
	registers := r.readRegisters
	writeRegisters := r.writeRegisters
	for k, v := range r.blockedMoves {
		// Pretend blocked moves are happening
		writeRegisters[k] = v
	}
	status := Status{
		RawRegisters: registers,
		Diag:         registers[0],
		RawAzPos:     int16(registers[1]),
		RawElPos:     int16(registers[2]),
		AzPos:        360 * float64(registers[1]) / 65536,
		ElPos:        regToSigned(registers[2]),
		AzVel:        regToSigned(registers[3]),
		ElVel:        regToSigned(registers[4]),

		WriteRegisters: writeRegisters,
		CommandDiag:    writeRegisters[0],
		CommandAzPos:   360 * float64(writeRegisters[1]) / 65536,
		CommandAzVel:   regToSigned(writeRegisters[2]),
		CommandElPos:   360 * float64(writeRegisters[4]) / 65536,
		CommandElVel:   regToSigned(writeRegisters[5]),
		CommandAzFlags: regToFlags(writeRegisters[3]),
		CommandElFlags: regToFlags(writeRegisters[6]),
	}
	for i := range status.Status {
		status.Status[i] = ((registers[5+(i/16)] >> (uint(i) % 16)) & 1) == 1
	}
	for i := range status.CommandStatus {
		status.CommandStatus[i] = ((writeRegisters[5+(i/8)] >> (uint(i) % 8)) & 1) == 1
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

	moving := len(r.blockedMoves) > 0 || ((status.CommandAzFlags != "NONE" || status.CommandElFlags != "NONE") && status.ShutdownError != 0) || math.Abs(status.AzVel) > QUIESCENT_VELOCITY || math.Abs(status.ElVel) > QUIESCENT_VELOCITY
	if moving {
		r.lastMove = time.Now()
	}
	status.Moving = time.Since(r.lastMove) < QUIESCENT_TIME
	status.MovingDisabled = r.blockedMoves != nil
	return status
}

type RCI struct {
	// acceptableShutdowns is a bitmask of the shutdown conditions that can be ignored
	acceptableShutdowns map[uint8]bool

	s              *serial.Port
	statusCallback rotator.StatusCallback
	mu             sync.Mutex
	readRegisters  [12]uint16
	writeRegisters [11]uint16
	lastDiag       uint16
	lastMove       time.Time
	// blockedMoves is non-nil when moves are being blocked
	blockedMoves map[int]uint16
}

func Connect(ctx context.Context, port string, statusCallback rotator.StatusCallback) (*RCI, error) {
	r := &RCI{statusCallback: statusCallback}
	go r.reconnectLoop(ctx, port)
	return r, nil
}
func (r *RCI) SetAcceptableShutdowns(value map[uint8]bool) {
	r.acceptableShutdowns = value
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
	exitingShutdown := false
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
			if status := r.parseRegisters(); status.ShutdownError != 0 && r.acceptableShutdowns[status.ShutdownError] {
				if !exitingShutdown {
					exitingShutdown = true
					log.Printf("Acceptable shutdown %d; automatically exiting shutdown", status.ShutdownError)
					r.exitShutdown()
				}
			} else {
				exitingShutdown = false
			}
		default:
			log.Printf("unknown input: %s", input)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("reading serial port: %v", err)
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
		if r.blockedMoves != nil {
			if register+i == 3 || register+i == 6 {
				if v == SERVO_NONE {
					delete(r.blockedMoves, register+i)
				} else {
					r.blockedMoves[register+i] = v
					v = SERVO_NONE
				}
			}
		}
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

func (r *RCI) SetMovingDisabled(blocked bool) {
	if blocked && r.blockedMoves == nil {
		r.mu.Lock()
		bm := map[int]uint16{
			3: r.writeRegisters[3],
			6: r.writeRegisters[6],
		}
		r.blockedMoves = bm
		r.mu.Unlock()
		for k, v := range bm {
			// Write will turn SERVO_* into SERVO_NONE
			r.Write(k, v)
		}
	} else if !blocked && r.blockedMoves != nil {
		r.mu.Lock()
		bm := r.blockedMoves
		r.blockedMoves = nil
		r.mu.Unlock()
		if len(bm) > 0 {
			r.lastDiag++
			r.Write(0, r.lastDiag)
			for k, v := range bm {
				r.Write(k, v)
			}
		}
	}
}

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
	r.exitShutdown()
}
func (r *RCI) exitShutdown() {
	// Toggling this bit from 0 to 1 to 0 in a time not less than
	// 0.1 seconds, but not greater than 1.0 second, will force
	// the RCI to exit from any prior shutdown condition. The
	// toggling feature prevents the bit from accidentally being
	// left active, since doing so would prevent genuine shutdowns
	// from proceeding normally.
	r.Write(10, 0)
	time.Sleep(200 * time.Millisecond)
	r.Write(10, 1)
	time.Sleep(200 * time.Millisecond)
	r.Write(10, 0)
}
