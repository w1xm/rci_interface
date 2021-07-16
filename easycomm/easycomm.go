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

type Status struct {
	// \?ENC command returns:
	RawAzPos int32
	RawElPos int32
	RawAzVel int32
	RawElVel int32
	// AZ command returns:
	AzPos float64
	// EL command returns:
	ElPos float64

	// \?VEL command returns:
	AzVel float64
	ElVel float64

	// \?DRV command returns:
	RawAzDrive int16
	RawElDrive int16

	// GS command returns:
	StatusRegister uint64
	// GE command returns
	ErrorRegister uint64

	// VE command returns:
	Version string

	// IP command returns:
	Status [8]bool
	// These are the flags broken down
	ElevationLower        bool
	ElevationUpper        bool
	AzimuthCW, AzimuthCCW bool

	HostOkay bool

	Moving bool

	// \?TGT comand returns:
	CommandAzPos, CommandElPos float64
	CommandAzVel, CommandElVel float64

	CommandAzFlags, CommandElFlags string
}

func ConnectTCP(ctx context.Context, port string, statusCallback StatusCallback) (*Rotator, error) {
	r := &Rotator{}
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
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading port: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		for {
			for _, cmd := range []string{
				`\?ENC`,
				`AZ`,
				`EL`,
				`\?VEL`,
				`GS`,
				`GE`,
				`VE`,
				`IP`,
				`\?TGT`,
			} {
				if _, err := r.conn.Write([]byte(cmd + "\n")); err != nil {
					return err
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
	// exitingShutdown := false
	// scanner := bufio.NewScanner(r.conn)
	// for scanner.Scan() {
	// 	input := scanner.Text()
	// 	if len(input) < 1 {
	// 		continue
	// 	}
	// 	switch {
	// 	case input[0] == '!':
	// 		log.Printf(input)
	// 	case input[0] == 'r':
	// 		r.mu.Lock()
	// 		for i, word := range strings.Split(input[1:len(input)-1], " ") {
	// 			v, err := strconv.ParseUint(word, 16, 16)
	// 			if err != nil {
	// 				log.Printf("failed to parse %q: %v", input, err)
	// 			}
	// 			r.readRegisters[i] = uint16(v)
	// 		}
	// 		r.notifyStatus()
	// 		r.mu.Unlock()
	// 		if status := r.parseRegisters(); status.ShutdownError != 0 && r.acceptableShutdowns[status.ShutdownError] {
	// 			if !exitingShutdown {
	// 				exitingShutdown = true
	// 				log.Printf("Acceptable shutdown %d; automatically exiting shutdown", status.ShutdownError)
	// 				r.exitShutdown()
	// 			}
	// 		} else {
	// 			exitingShutdown = false
	// 		}
	// 	default:
	// 		log.Printf("unknown input: %s", input)
	// 	}
	// }
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
			*dir = regToFlags(r.status.StatusRegister >> (i * 8) & 0xFF)
		}
		r.status.Moving = (r.status.StatusRegister & 0x22) != 0
	case input[:2] == "GE": // GExxx
		i, err := strconv.ParseInt(input[2:], 10, 64)
		if err != nil {
			return err
		}
		r.status.ErrorRegister = uint64(i)
	case input[:2] == "VE": // VEaaaaaa
		r.status.Version = input[2:]
	case input[:2] == "IP": // IPn,n
		parts := strings.Split(input[2:], ",")
		i, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Printf("%q: %v", input, err)
			return nil
		}
		for j := 0; i+j < len(r.status.Status) && j < len(parts); j++ {
			r.status.Status[i+j] = (parts[j] == "1")
		}
		r.status.ElevationLower = r.status.Status[0]
		r.status.ElevationUpper = r.status.Status[1]
		r.status.AzimuthCW = r.status.Status[2]
		r.status.AzimuthCCW = r.status.Status[3]
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
	case input[:5] == `\?TGT`:
		return parseFloatArray([]*float64{
			&r.status.CommandAzPos,
			&r.status.CommandElPos,
			&r.status.CommandAzVel,
			&r.status.CommandElVel,
		}, input[5:])
	case input[:5] == `\?DRV`:
		parts := strings.Split(input[5:], ",")
		for i, field := range []*int16{
			&r.status.RawAzDrive,
			&r.status.RawElDrive,
		} {
			i, err := strconv.Atoi(parts[i])
			if err != nil {
				return err
			}
			*field = int16(i)
		}
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
	dir := "U"
	if angle < 0 {
		angle = -angle
		dir = "D"
	}
	r.send(fmt.Sprintf("V%s%03.0f", dir, angle*1000))
}

func (r *Rotator) SetElevationVelocity(angle float64) {
	dir := "R"
	if angle < 0 {
		angle = -angle
		dir = "L"
	}
	r.send(fmt.Sprintf("V%s%03.0f", dir, angle*1000))
}
