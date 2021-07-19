package simulator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/w1xm/rci_interface/easycomm/internal/status"
	"golang.org/x/sync/errgroup"
)

// Loosely inspired by https://github.com/rolandturner/ground-simulator/blob/master/Simulator.js

type Simulator struct {
	conn   io.ReadWriteCloser
	mu     sync.Mutex
	status status.Status
	last   status.Status
}

func New() (*Simulator, net.Conn) {
	a, b := net.Pipe()
	return &Simulator{conn: a, status: status.Status{Version: "sim"}}, b
}

var cmdRE = regexp.MustCompile(`^([\?A-Z]+)(.*)$`)

func (s *Simulator) parseInput(input string) error {
	parts := cmdRE.FindStringSubmatch(input)
	if parts == nil {
		return fmt.Errorf("unrecognized command %q", input)
	}
	cmd, parts := parts[1], strings.Split(parts[2], ",")
	if len(parts) == 1 && parts[0] == "" {
		parts = nil
	}
	switch cmd {
	case "SA":
		s.status.CommandAzFlags = "NONE"
		return nil
	case "SE":
		s.status.CommandElFlags = "NONE"
		return nil
	case "AZ":
		if len(parts) > 0 {
			s.status.CommandAzFlags = "POSITION"
			return status.ParseFloat(&s.status.CommandAzPos, parts[0])
		}
	case "EL":
		if len(parts) > 0 {
			s.status.CommandElFlags = "POSITION"
			return status.ParseFloat(&s.status.CommandElPos, parts[0])
		}
	case "VU", "VD":
		if len(parts) > 0 {
			s.status.CommandElFlags = "VELOCITY"
			status.ParseFloat(&s.status.CommandElVel, parts[0])
			// Velocity commands are in mdeg/s
			s.status.CommandElVel /= 1000
			if cmd[1] == 'D' {
				s.status.CommandElVel = -s.status.CommandElVel
			}
		} else {
			dir := "U"
			if s.status.CommandElVel < 0 {
				dir = "D"
			}
			s.send("V%s%3.2f", dir, math.Abs(s.status.CommandElVel))
		}
		return nil
	case "VL", "VR":
		if len(parts) > 0 {
			s.status.CommandAzFlags = "VELOCITY"
			status.ParseFloat(&s.status.CommandAzVel, parts[0])
			// Velocity commands are in mdeg/s
			s.status.CommandAzVel /= 1000
			if cmd[1] == 'L' {
				s.status.CommandAzVel = -s.status.CommandAzVel
			}
		} else {
			dir := "R"
			if s.status.CommandAzVel < 0 {
				dir = "L"
			}
			s.send("V%s%3.2f", dir, math.Abs(s.status.CommandAzVel))
		}
		return nil
	}
	if len(parts) == 0 {
		return s.sendStatus(nil, cmd)
	}
	return fmt.Errorf("unknown command %q %+v", cmd, parts)
}

const (
	// Maximum acceleration in degrees/second^2
	maxAccel = 30
	// Maximum velocity in degrees/second
	maxVel = 30
	minVel = 0.1
	// Acceleration due to drag when not driving
	dragAccel = 30
	// Discrete simulation step size
	stepSize = 25 * time.Millisecond
)

func (s *Simulator) Run(ctx context.Context) error {
	defer s.conn.Close()
	t := time.NewTicker(stepSize)
	defer t.Stop()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
			}
			if err := s.step(); err != nil {
				return err
			}
		}
	})
	g.Go(s.reader)
	return g.Wait()
}

func (s *Simulator) reader() error {
	scanner := bufio.NewScanner(s.conn)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		input := scanner.Text()
		log.Printf("srv->sim: %s", input)
		if err := s.parseInput(input); err != nil {
			log.Printf("parsing %q: %v", input, err)
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading port: %w", err)
	}
	return nil
}

// posServo returns a target velocity for the given move
func posServo(s, t float64) float64 {
	// TODO: PID control to prevent overshoot
	move := math.Remainder(t-s, 360)
	delta := 2 * math.Abs(move)
	if delta > maxVel {
		delta = maxVel
	}
	if move < 0 {
		delta = -delta
	}
	return delta
}

// velServo returns an actual velocity for the given current and target velocity
func velServo(s, t float64) float64 {
	delta := math.Abs(t - s)
	if delta > maxAccel*stepSize.Seconds() {
		delta = maxAccel * stepSize.Seconds()
	}
	if t < s {
		delta = -delta
	}
	new := s + delta
	if math.Abs(new) < minVel {
		return 0
	}
	if new > maxVel {
		return maxVel
	} else if new < -maxVel {
		return -maxVel
	}
	return new
}

func drag(s float64) float64 {
	a := math.Abs(s)
	a -= dragAccel * stepSize.Seconds()
	if a < 0 {
		a = 0
	}
	if s < 0 {
		return -a
	}
	return a
}

func (s *Simulator) step() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if serr := s.sendStatus(&s.last, ""); serr != nil {
			log.Printf("sending status: %v", err)
			if err == nil {
				err = serr
			}
		}
		s.last = s.status
	}()
	// Update position
	cmdVel := s.status.CommandAzVel
	azStatus := 0
	switch s.status.CommandAzFlags {
	case "POSITION":
		cmdVel = posServo(s.status.AzPos, s.status.CommandAzPos)
		azStatus = 4
		fallthrough
	case "VELOCITY":
		azStatus |= 2
		s.status.AzVel = velServo(s.status.AzVel, cmdVel)
	default:
		// Coasting
		azStatus = 1
		s.status.AzVel = drag(s.status.AzVel)
	}
	cmdVel = s.status.CommandElVel
	elStatus := 0
	switch s.status.CommandElFlags {
	case "POSITION":
		cmdVel = posServo(s.status.ElPos, s.status.CommandElPos)
		elStatus = 4
		fallthrough
	case "VELOCITY":
		s.status.ElVel = velServo(s.status.ElVel, cmdVel)
		elStatus |= 2
	default:
		// Coasting
		elStatus = 1
		s.status.ElVel = drag(s.status.ElVel)
	}

	s.status.AzPos = math.Mod(s.status.AzPos+s.status.AzVel*stepSize.Seconds()+360, 360)
	s.status.ElPos = math.Mod(s.status.ElPos+s.status.ElVel*stepSize.Seconds()+360, 360)
	s.status.ElevationLimit = 0
	if s.status.ElPos > 180 {
		s.status.ElPos = 0
		s.status.ElVel = 0
		s.status.ElevationLimit = 1
	} else if s.status.ElPos > 90 {
		s.status.ElPos = 90
		s.status.ElVel = 0
		s.status.ElevationLimit = 2
	}

	s.status.StatusRegister = uint64(azStatus + (elStatus << 8))
	return nil
}

func (s *Simulator) sendStatus(old *status.Status, cmd string) error {
	var oldv reflect.Value
	if old != nil {
		oldv = reflect.ValueOf(*old)
	}
	v := reflect.ValueOf(s.status)
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		tag := field.Tag.Get("report")
		if tag == "" || tag == "-" {
			continue
		}
		fv := v.Field(i)
		value := fv.Interface()
		if (cmd != "" && cmd != tag) || (cmd == "" && old != nil && reflect.DeepEqual(value, oldv.Field(i).Interface())) {
			continue
		}
		switch fv.Kind() {
		case reflect.Float32, reflect.Float64:
			if err := s.send("%s%3.2f", tag, value); err != nil {
				return err
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if err := s.send("%s%d", tag, value); err != nil {
				return err
			}
		case reflect.Bool:
			if fv.Bool() {
				if err := s.send("%s1", tag, value); err != nil {
					return err
				}
			} else {
				if err := s.send("%s0", tag, value); err != nil {
					return err
				}
			}
		case reflect.String:
			if err := s.send("%s%s", tag, value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("don't know how to send %s: %q (value %+v)", field.Name, tag, fv.Interface())
		}
	}
	return nil
}

func (s *Simulator) send(cmd string, fields ...interface{}) error {
	if len(fields) > 0 {
		cmd = fmt.Sprintf(cmd, fields...)
	}
	log.Printf("sim->srv: %s", cmd)
	_, err := fmt.Fprintf(s.conn, "%s\n", cmd)
	return err
}
