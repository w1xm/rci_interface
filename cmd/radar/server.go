package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pebbe/novas"
	"github.com/w1xm/rci_interface/rci"
)

type Server struct {
	place  *novas.Place
	mu     sync.Mutex
	r      *rci.RCI
	bodies []*novas.Body

	statusMu   sync.RWMutex
	statusCond *sync.Cond
	status     rci.Status
}

func NewServer(ctx context.Context, port string, place *novas.Place) (*Server, error) {
	s := &Server{place: place}
	s.statusCond = sync.NewCond(s.statusMu.RLocker())
	r, err := rci.Connect(ctx, *serialPort, s.statusCallback)
	if err != nil {
		return nil, err
	}
	s.r = r
	s.bodies = []*novas.Body{
		novas.Sun(),
		novas.Moon(),
		novas.Mercury(),
		novas.Venus(),
		novas.Mars(),
		novas.Jupiter(),
		novas.Saturn(),
		novas.Uranus(),
		novas.Neptune(),
	}
	return s, nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (s *Server) StatusHandler(w http.ResponseWriter, r *http.Request) {
	s.statusMu.RLock()
	status := s.status
	s.statusMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(status)
	if err != nil {
		log.Print(err)
		return
	}
	w.Write(data)
}

type Command struct {
	Command  string  `json:"command"`
	Register int     `json:"register"`
	Value    uint16  `json:"value"`
	Position float64 `json:"position"`
	Velocity float64 `json:"velocity"`
}

func (s *Server) StatusSocketHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Read and process incoming messages
	go func() {
		for {
			var msg Command
			if err := conn.ReadJSON(&msg); err != nil {
				log.Printf("parsing json: %v", err)
				cancel()
				conn.Close()
				break
			}
			s.mu.Lock()
			switch msg.Command {
			case "write":
				s.r.Write(msg.Register, msg.Value)
			case "set_azimuth_position":
				s.r.SetAzimuthPosition(msg.Position)
			case "set_elevation_position":
				s.r.SetElevationPosition(msg.Position)
			case "stop":
				s.r.Stop()
			default:
				log.Printf("Unknown command: %+v", msg)
			}
			s.mu.Unlock()
		}
	}()

	send := func(status rci.Status) {
		data, err := json.Marshal(status)
		if err != nil {
			log.Print(err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Print(err)
			return
		}
	}

	s.statusMu.RLock()
	status := s.status
	s.statusMu.RUnlock()
	send(status)

	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.statusCond.Wait()
		status := s.status
		send(status)
	}
}

func (s *Server) statusCallback(status rci.Status) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status = status
	s.statusCond.Broadcast()
}
