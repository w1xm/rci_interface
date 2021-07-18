package easycomm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type StatusCallback func(status Status)

// Rotator implements support for an EasyComm III rotator
type Rotator struct {
	conn io.ReadWriteCloser

	supportsEnc bool
	supportsVel bool

	statusCallback StatusCallback
	mu             sync.Mutex
	status         Status
}

// Protocol docs at https://github.com/Hamlib/Hamlib/blob/master/rotators/easycomm/easycomm.txt
// Compatible with https://gitlab.com/Quartapound/satnogs-rotator-firmware/-/blob/master/libraries/easycomm.h

type Status struct {
	// \?ENC command returns:
	RawAzPos int32
	RawElPos int32
	RawAzVel int32
	RawElVel int32

	// AZ command returns:
	AzPos float64 `report:"AZ"`
	// EL command returns:
	ElPos float64 `report:"EL"`

	// IP0 returns temperature
	Temperature float64 `report:"IP0,"`

	// IP1 returns Az endstop
	AzimuthCCW, AzimuthCW bool
	// IP2 returns El endstop
	ElevationLower, ElevationUpper bool

	// IP3 returns Az position (redundant)
	// IP4 returns El position (redundant)

	// IP5 returns Az drive load
	RawAzDrive float64
	// IP6 returns El drive load
	RawElDrive float64
	// IP7 returns Az speed
	AzVel float64 `report:"IP7,"`
	// IP8 returns El speed
	ElVel float64 `report:"IP8,"`

	// CR1-3 are azimuth P-I-D
	// CR4-6 are elevation P-I-D
	// CR7 is azimuth park position
	// CR8 is elevation park position
	// CR10-13 return Az position setpoint, El position setpoint, Az velocity setpoint, El velocity setpoint (not supported by SatNOGS)
	CommandAzPos float64 `report:"CR10,"`
	CommandElPos float64 `report:"CR11,"`
	CommandAzVel float64 `report:"CR12,"`
	CommandElVel float64 `report:"CR13,"`

	// GS command returns:
	StatusRegister uint64 `report:"GS"`
	// GE command returns
	ErrorRegister uint64 `report:"GE"`
	ErrorFlags    struct {
		NoError     bool
		SensorError bool
		HomingError bool
		MotorError  bool
	}

	// VE command returns:
	Version string `report:"VE"`

	HostOkay bool

	Moving bool

	CommandAzFlags, CommandElFlags string
}

func ConnectTCP(ctx context.Context, port string, statusCallback StatusCallback) (*Rotator, error) {
	r := &Rotator{statusCallback: statusCallback}
	go r.reconnectLoop(ctx, port)
	return r, nil
}

func (r *Rotator) reconnectLoop(ctx context.Context, port string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
		dialer := &net.Dialer{
			Timeout: time.Second,
		}
		conn, err := dialer.DialContext(ctx, "tcp", port)
		if err != nil {
			log.Printf("opening %q: %v", port, err)
			continue
		}
		log.Printf("opened %q", port)
		r.mu.Lock()
		r.conn = conn
		r.mu.Unlock()
		r.watch(ctx)
		r.mu.Lock()
		r.conn = nil
		r.mu.Unlock()
	}
}

func parseFloat(dest *float64, input string) error {
	f, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return err
	}
	*dest = f
	return nil
}

func parseFloatArray(dest []*float64, input string) error {
	parts := strings.Split(input, ",")
	for i, field := range dest {
		if i >= len(parts) {
			return errors.New("truncated list")
		}
		if err := parseFloat(field, parts[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Rotator) watch(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Wait for context to be canceled, then close connection.
		<-ctx.Done()
		return r.conn.Close()
	})

	g.Go(func() error {
		scanner := bufio.NewScanner(r.conn)
		scanner.Split(bufio.ScanWords)
		for scanner.Scan() {
			input := scanner.Text()
			if err := r.parseInput(input); err != nil {
				log.Printf("parsing %q: %v", input, err)
				continue
			}
		}
		if err := scanner.Err(); err != nil && err != io.ErrClosedPipe {
			return fmt.Errorf("reading port: %w", err)
		}
		// Return EOF so errgroup cancels the context.
		return io.EOF
	})
	g.Go(func() error {
		for {
			for _, cmd := range []string{
				`\?ENC`,
				`AZ`,
				`EL`,
				`GS`,
				`GE`,
				`VE`,
				`IP`,
			} {
				if _, err := r.conn.Write([]byte(cmd + "\n")); err != nil {
					if err == io.EOF || err == io.ErrClosedPipe {
						return nil
					}
					return fmt.Errorf("writing %q: %v", cmd, err)
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
	})
	return g.Wait()
}

func regToFlags(reg uint64) string {
	switch reg {
	case 1:
		return "NONE"
	case 2:
		return "VELOCITY"
	case 4, 6:
		return "POSITION"
	case 8:
		return "ERROR"
	}
	return fmt.Sprintf("UNKNOWN(%d)", reg)
}

func (r *Rotator) parseInput(input string) error {
	if len(input) < 2 {
		return errors.New("truncated output")
	}
	r.mu.Lock()
	old := r.status
	defer func() {
		new := r.status
		r.mu.Unlock()
		if new != old {
			r.notifyStatus()
		}
	}()
	switch {
	case input[:2] == "AZ": // AZxxx.x
		return parseFloat(&r.status.AzPos, input[2:])
	case input[:2] == "EL": // ELxxx.x
		return parseFloat(&r.status.ElPos, input[2:])
	case input[:2] == "GS": // GSxxx
		i, err := strconv.ParseInt(input[2:], 10, 64)
		if err != nil {
			return err
		}
		r.status.StatusRegister = uint64(i)
		for i, dir := range []*string{&r.status.CommandAzFlags, &r.status.CommandElFlags} {
			val := r.status.StatusRegister >> (i * 8) & 0xFF
			if val == 0 {
				val = r.status.StatusRegister & 0xFF
			}
			*dir = regToFlags(val)
		}
		r.status.Moving = (r.status.StatusRegister & 0x202) != 0
	case input[:2] == "GE": // GExxx
		i, err := strconv.ParseInt(input[2:], 10, 64)
		if err != nil {
			return err
		}
		r.status.ErrorRegister = uint64(i)
		for i, v := range []*bool{
			&r.status.ErrorFlags.NoError,
			&r.status.ErrorFlags.SensorError,
			&r.status.ErrorFlags.HomingError,
			&r.status.ErrorFlags.MotorError,
		} {
			*v = r.status.ErrorRegister&(1<<i) == (1 << i)
		}
	case input[:2] == "VE": // VEaaaaaa
		r.status.Version = input[2:]
	case input[:2] == "IP": // IPn,n
		parts := strings.Split(input[2:], ",")
		i, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Printf("%q: %v", input, err)
			return nil
		}
		parts = parts[1:]
		for j := 0; j < len(parts); j++ {
			valueFloat, _ := strconv.ParseFloat(parts[j], 64)
			valueInt, _ := strconv.Atoi(parts[j])
			switch i + j {
			case 0:
				r.status.Temperature = valueFloat
			case 1:
				r.status.AzimuthCCW = valueInt&1 == 1
				r.status.AzimuthCW = valueInt&2 == 2
			case 2:
				r.status.ElevationLower = valueInt&1 == 1
				r.status.ElevationUpper = valueInt&2 == 2
			case 5:
				r.status.RawAzDrive = valueFloat
			case 6:
				r.status.RawElDrive = valueFloat
			case 7:
				r.status.AzVel = valueFloat
			case 8:
				r.status.ElVel = valueFloat
			}
		}
	case input[:2] == "CR": // CRn,nparts := strings.Split(input[2:], ",")
		parts := strings.Split(input[2:], ",")
		i, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Printf("%q: %v", input, err)
			return nil
		}
		parts = parts[1:]
		for j := 0; j < len(parts); j++ {
			valueFloat, _ := strconv.ParseFloat(parts[j], 64)
			//valueInt, _ := strconv.Atoi(parts[j])
			switch i + j {
			case 10:
				r.status.CommandAzPos = valueFloat
			case 11:
				r.status.CommandElPos = valueFloat
			case 12:
				r.status.CommandAzVel = valueFloat
			case 13:
				r.status.CommandElVel = valueFloat
			}
		}
	case len(input) < 5:
		return errors.New("unknown rotator output")
	case input[:5] == `\?ENC`:
		parts := strings.Split(input[5:], ",")
		for i, field := range []*int32{
			&r.status.RawAzPos,
			&r.status.RawElPos,
			&r.status.RawAzVel,
			&r.status.RawElVel,
		} {
			i, err := strconv.Atoi(parts[i])
			if err == nil {
				*field = int32(i)
			}
		}
	case input[:5] == `\?VEL`:
		return parseFloatArray([]*float64{
			&r.status.AzVel,
			&r.status.ElVel,
		}, input[5:])
	default:
		return errors.New("unknown rotator output")
	}
	return nil
}

func (r *Rotator) notifyStatus() {
	//status := r.parseRegisters()
	r.mu.Lock()
	status := r.status
	r.mu.Unlock()
	r.statusCallback(status)
}

func (r *Rotator) send(cmd string) error {
	if _, err := r.conn.Write([]byte(cmd + "\n")); err != nil {
		return err
	}
	return nil
}

func (r *Rotator) Stop() {
	r.send("SA SE")
}

func (r *Rotator) SetAzimuthPosition(angle float64) {
	r.send(fmt.Sprintf("AZ%03.1f", angle))
}

func (r *Rotator) SetElevationPosition(angle float64) {
	r.send(fmt.Sprintf("EL%03.1f", angle))
}

func (r *Rotator) SetAzimuthVelocity(angle float64) {
	dir := "R"
	if angle < 0 {
		angle = -angle
		dir = "L"
	}
	r.send(fmt.Sprintf("V%s%03.0f", dir, angle*1000))
}

func (r *Rotator) SetElevationVelocity(angle float64) {
	dir := "U"
	if angle < 0 {
		angle = -angle
		dir = "D"
	}
	r.send(fmt.Sprintf("V%s%03.0f", dir, angle*1000))
}
