package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pebbe/novas"
	"github.com/w1xm/rci_interface/rci"
)

type Status struct {
	rci.Status
	CommandTrackingBody int
	Bodies              []string
	OffsetAz, OffsetEl  float64
}

type Server struct {
	place  *novas.Place
	mu     sync.Mutex
	r      *rci.Offset
	bodies []*novas.Body

	statusMu   sync.RWMutex
	statusCond *sync.Cond
	status     Status
}

func NewServer(ctx context.Context, port string, place *novas.Place) (*Server, error) {
	s := &Server{place: place}
	s.statusCond = sync.NewCond(s.statusMu.RLocker())
	r, err := rci.ConnectOffset(ctx, *serialPort, s.statusCallback, 0, 0)
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
		novas.Pluto(),
		novas.NewStar(
			"Polaris", "HR", 424,
			37.95456067, 89.26410897,
			44.48, -11.85,
			7.54, -16.42,
		),
	}
	s.updateBodies()
	go s.trackLoop(ctx)
	return s, nil
}

// updateBodies syncs s.status.Bodies with s.bodies.
// It must be called with statusMu locked.
func (s *Server) updateBodies() {
	s.status.Bodies = []string{"NONE"}
	for _, b := range s.bodies {
		s.status.Bodies = append(s.status.Bodies, b.Name())
	}
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
	Body     int     `json:"body"`
	Star     *Star   `json:"star"`
}

type Star struct {
	StarName       string  `json:"starname"`       // name of celestial object
	Catalog        string  `json:"catalog"`        // catalog designator (e.g., HIP)
	StarNumber     int64   `json:"starnumber"`     // integer identifier assigned to object
	RA             float64 `json:"ra"`             // ICRS right ascension (hours)
	Dec            float64 `json:"dec"`            // ICRS declination (degrees)
	ProMoRA        float64 `json:"promora"`        // ICRS proper motion in right ascension (milliarcseconds/year)
	ProMoDec       float64 `json:"promodec"`       // ICRS proper motion in declination (milliarcseconds/year)
	Parallax       float64 `json:"parallax"`       // parallax (milliarcseconds)
	RadialVelocity float64 `json:"radialvelocity"` // radial velocity (km/s)
}

func (s *Server) trackLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(250 * time.Millisecond):
		}
		s.mu.Lock()
		s.statusMu.RLock()
		command := s.status.CommandTrackingBody
		s.statusMu.RUnlock()
		if command > 0 && command <= len(s.bodies) {
			body := s.bodies[command-1]
			topo := body.Topo(novas.Now(), s.place, novas.REFR_NONE)
			s.r.SetAzimuthPosition(topo.Az)
			s.r.SetElevationPosition(topo.Alt)
		}
		s.mu.Unlock()
	}
}

func (s *Server) track(body int) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.CommandTrackingBody = body
}

func (s *Server) StatusSocketHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
			case "track":
				s.track(msg.Body)
			case "write":
				s.r.Write(msg.Register, msg.Value)
			case "set_azimuth_position":
				s.track(0)
				s.r.SetAzimuthPosition(msg.Position)
			case "set_elevation_position":
				s.track(0)
				s.r.SetElevationPosition(msg.Position)
			case "set_azimuth_velocity":
				s.track(0)
				s.r.SetAzimuthVelocity(msg.Velocity)
			case "set_elevation_velocity":
				s.track(0)
				s.r.SetElevationVelocity(msg.Velocity)
			case "stop":
				s.track(0)
				s.r.Stop()
			case "stop_hard":
				s.track(0)
				s.r.SetAzimuthVelocity(0)
				s.r.SetElevationVelocity(0)
			case "set_azimuth_offset":
				s.status.OffsetAz = msg.Position
				s.r.SetAzimuthOffset(s.status.OffsetAz)
			case "set_elevation_offset":
				s.status.OffsetEl = msg.Position
				s.r.SetElevationOffset(s.status.OffsetEl)
			case "add_star":
				s.statusMu.Lock()
				s.bodies = append(s.bodies, novas.NewStar(
					msg.Star.StarName,
					msg.Star.Catalog,
					msg.Star.StarNumber,
					msg.Star.RA,
					msg.Star.Dec,
					msg.Star.ProMoRA,
					msg.Star.ProMoDec,
					msg.Star.Parallax,
					msg.Star.RadialVelocity))
				s.updateBodies()
				s.statusMu.Unlock()
			default:
				log.Printf("Unknown command: %+v", msg)
			}
			s.mu.Unlock()
		}
	}()

	send := func(status Status) {
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

	c := make(chan struct{}, 1)
	go func() {
		s.statusMu.RLock()
		defer s.statusMu.RUnlock()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			s.statusCond.Wait()
			status = s.status
			select {
			case c <- struct{}{}:
			default:
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c:
			send(status)
			select {
			case <-ctx.Done():
				return
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
}

func (s *Server) statusCallback(status rci.Status) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.Status = status
	s.statusCond.Broadcast()
}
