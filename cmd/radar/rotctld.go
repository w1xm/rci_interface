package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

func (s *Server) ListenRotctld(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		log.Print("shutdown; closing rotctld socket")
		ln.Close()
	}()
	go func() {
		for ctx.Err() == nil {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("failed to accept: %v", err)
				continue
			}
			go s.handleRotctld(conn)
		}
	}()
	return nil
}

func (s *Server) handleRotctld(conn net.Conn) {
	defer conn.Close()
	log.Printf("accepted connection from %v", conn.RemoteAddr())
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Two forms of command: single character, or "+\" followed by command name.
		cmd := scanner.Text()
		var args []string
		var extended bool
		if len(cmd) == 0 {
			continue
		} else if len(cmd) > 2 && cmd[0:2] == `+\` {
			extended = true
			parts := strings.Split(cmd, " ")
			cmd = parts[0][2:len(parts[0])]
			if len(parts) > 1 {
				args = parts[1:len(parts)]
			}
			fmt.Fprintf(conn, "%s:\n", cmd)
		} else {
			// Space after command is optional.
			if len(cmd) > 1 {
				args = strings.Fields(strings.TrimLeft(cmd[1:len(cmd)], " "))
			}
			cmd = string(cmd[0])
		}
		log.Printf("%v command: %q args: %#v", conn.RemoteAddr(), cmd, args)
		rprt := -1
		switch cmd {
		case "1", "dump_caps":
			fmt.Fprintf(conn, `Model name: RCI
Mfg name: Sigmet
Rot type: Az-El
Min Azimuth: -180.00
Max Aximuth: 180.00
Min Elevation: 0.00
Max Elevation: 90.00
Can set Position: Y
Can get Position: Y
Can Stop: Y
Can Park: N
Can Reset: N
Can Move: Y
Can get Info: N
`)
			rprt = 0
		case "S", "stop":
			extended = true // always print RPRT
			s.mu.Lock()
			s.track(0)
			s.r.Stop()
			s.mu.Unlock()
			rprt = 0
		case "P", "set_pos":
			extended = true // always print RPRT
			if len(args) != 2 {
				rprt = -22
				break
			}
			az, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				rprt = -22
				break
			}
			el, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				rprt = -22
				break
			}
			s.mu.Lock()
			s.track(0)
			s.r.SetAzimuthPosition(az)
			s.r.SetElevationPosition(el)
			s.mu.Unlock()
			rprt = 0
		case "M", "move":
			extended = true // always print RPRT
			if len(args) != 2 {
				rprt = -22
				break
			}
			dir, err := strconv.Atoi(args[0])
			if err != nil {
				rprt = -22
				break
			}
			// Speed is 0-100. We divide by 10 to get deg/sec.
			speed, err := strconv.Atoi(args[1])
			if err != nil {
				rprt = -22
				break
			}
			switch dir {
			case 2: // Up
				speed *= -1
				fallthrough
			case 4: // Down
				s.mu.Lock()
				s.track(0)
				s.r.SetElevationVelocity(float64(speed) / 10)
				s.mu.Unlock()
				rprt = 0
			case 8: // Left
				speed *= -1
				fallthrough
			case 16: // Right
				s.mu.Lock()
				s.track(0)
				s.r.SetAzimuthVelocity(float64(speed) / 10)
				s.mu.Unlock()
				rprt = 0
			default:
				rprt = -22
			}
		case "p", "get_pos":
			s.statusMu.RLock()
			status := s.status
			s.statusMu.RUnlock()
			az := status.AzimuthPosition()
			if az > 180 {
				az -= 360
			}
			if extended {
				fmt.Fprintf(conn, "Azimuth: %.6f\nElevation: %.6f\n", az, status.ElevationPosition())
			} else {
				fmt.Fprintf(conn, "%.6f\n%.6f\n", az, status.ElevationPosition())
			}
			rprt = 0
		}
		if extended || rprt != 0 {
			fmt.Fprintf(conn, "RPRT %d\n", rprt)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("reading from %v: %v", conn.RemoteAddr(), err)
	}
}
