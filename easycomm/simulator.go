package easycomm

import (
	"context"
	"fmt"
	"log"
	"math"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Loosely inspired by https://github.com/rolandturner/ground-simulator/blob/master/Simulator.js

type Simulator struct {
	mu     sync.Mutex
	status Status
}

var cmdRE = regexp.MustCompile(`^([\?A-Z]+)(.*)$`)

func (s *Simulator) parseInput(input string) error {
	parts := cmdRE.FindStringSubmatch(input)
	if parts == nil {
		return fmt.Errorf("unrecognized command %q", input)
	}
	cmd, parts := parts[0], strings.Split(parts[1], ",")
	if len(parts) == 1 && parts[0] == "" {
		parts = nil
	}
	switch cmd {
	case "AZ":
		if len(parts) > 0 {
			parseFloat(&s.status.CommandAzPos, parts[0])
		} else {
			s.send("AZ%3.2f", s.status.AzPos)
		}
	case "EL":
		if len(parts) > 0 {
			parseFloat(&s.status.CommandElPos, parts[0])
		} else {
			s.send("EL%3.2f", s.status.ElPos)
		}
	case "VU", "VD":
		if len(parts) > 0 {
			parseFloat(&s.status.CommandElVel, parts[0])
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
	case "VL", "VR":
		if len(parts) > 0 {
			parseFloat(&s.status.CommandAzVel, parts[0])
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
	}
	return nil
}

const (
	// Maximum acceleration in degrees/second^2
	maxAccel = 30
	// Maximum velocity in degrees/second
	maxVel = 30
	// Acceleration due to drag when not driving
	dragAccel = 30
	// Discrete simulation step size
	stepSize = 100 * time.Millisecond
)

func (s *Simulator) Run(ctx context.Context) error {
	t := time.NewTicker(stepSize)
	defer t.Stop()
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
	return nil
}

// posServo returns a target velocity for the given move
func posServo(s, t float64) float64 {
	// TODO: PID control to prevent overshoot
	delta := math.Abs(t - s)
	if delta > maxVel {
		delta = maxVel
	}
	if t < s {
		delta = -delta
	}
	return s + delta
}

func velServo(s, t float64) float64 {
	delta := math.Abs(t - s)
	if delta > maxAccel {
		delta = maxAccel
	}
	delta *= stepSize.Seconds()
	if t < s {
		delta = -delta
	}
	new := t + delta
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
	if a < 0 {
		return -a
	}
	return a
}

func (s *Simulator) step() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.status
	defer s.sendStatus(&old)
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

	s.status.StatusRegister = uint64(azStatus + (elStatus << 8))
	return nil
}

func (s *Simulator) sendStatus(old *Status) error {
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
		if old != nil && reflect.DeepEqual(value, oldv.Field(i).Interface()) {
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
		default:
			return fmt.Errorf("don't know how to send %s: %q (value %+v)", field.Name, tag, fv.Interface())
		}
	}
	return nil
}

func (s *Simulator) send(cmd string, fields ...interface{}) error {
	cmd += "\n"
	if len(fields) > 0 {
		cmd = fmt.Sprintf(cmd, fields...)
	}
	log.Print(cmd)
	return nil
}
