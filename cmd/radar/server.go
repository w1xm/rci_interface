package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pebbe/novas"
	"github.com/w1xm/rci_interface/rci"
	"github.com/w1xm/rci_interface/sequencer"
)

type AuthorizedClient struct {
	RemoteAddr string
	Name       string
}

type Status struct {
	SequenceNumber int
	rci.Status
	Sequencer           sequencer.Status
	CommandTrackingBody int
	Bodies              []string
	OffsetAz, OffsetEl  float64
	// Authorized is true if the current connection is allowed to mutate state.
	Authorized          bool
	AuthorizedClients   []AuthorizedClient
	Latitude, Longitude float64
}

func (s Status) Clone() Status {
	s.Bodies = append([]string{}, s.Bodies...)
	s.AuthorizedClients = append([]AuthorizedClient{}, s.AuthorizedClients...)
	return s
}

func (s *Status) AddAuthorizedClient(c AuthorizedClient) {
	s.AuthorizedClients = append(s.AuthorizedClients, c)
}

func (s *Status) RemoveAuthorizedClient(c AuthorizedClient) {
	for i, c2 := range s.AuthorizedClients {
		if c2 == c {
			s.AuthorizedClients = append(s.AuthorizedClients[:i], s.AuthorizedClients[i+1:]...)
			return
		}
	}
}

type Server struct {
	passwords []string
	place     *novas.Place
	mu        sync.Mutex
	r         *rci.Offset
	bodies    []*novas.Body
	seq       *sequencer.Sequencer

	statusMu   sync.RWMutex
	statusCond *sync.Cond
	status     Status
}

func NewServer(ctx context.Context, port string, passwords []string, latitude, longitude float64, place *novas.Place, azOffset, elOffset float64, sequencerURL string, sequencerPort string, sequencerBaud int) (*Server, error) {
	s := &Server{
		status: Status{
			Latitude:  latitude,
			Longitude: longitude,
		},
		place:     place,
		passwords: passwords,
	}
	s.statusCond = sync.NewCond(s.statusMu.RLocker())
	r, err := rci.ConnectOffset(ctx, port, s.statusCallback, azOffset, elOffset)
	if err != nil {
		return nil, err
	}
	s.r = r
	if sequencerURL != "" {
		s.seq, err = sequencer.ConnectRemote(ctx, sequencerURL, s.sequencerStatusCallback)
	} else {
		s.seq, err = sequencer.Connect(ctx, sequencerPort, sequencerBaud, s.sequencerStatusCallback)
	}
	if err != nil {
		return nil, err
	}
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
			37.95456067/15, 89.26410897,
			44.48, -11.85,
			7.54, -16.42,
		),
		novas.NewStar(
			"Vega", "HR", 7001,
			279.23473479/15, 38.78368896,
			200.94, 286.23,
			130.23, -20.60,
		),
		novas.NewStar(
			"Cygnus A", "W", 57,
			299.86815263/15, 40.73391583,
			0, 0,
			0, 16360,
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
	status := s.status.Clone()
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
	Command        string  `json:"command"`
	SequenceNumber int     `json:"seq"`
	Register       int     `json:"register"`
	Value          uint16  `json:"value"`
	Position       float64 `json:"position"`
	Velocity       float64 `json:"velocity"`
	Body           int     `json:"body"`
	Star           *Star   `json:"star"`
	Band           int     `json:"band"`
	Enabled        bool    `json:"enabled"`
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

func isLocal(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	switch host {
	case "127.0.0.1", "::1":
		return true
	}
	return false
}

func (s *Server) isAuth(r *http.Request) (string, bool) {
	protocols := websocket.Subprotocols(r)
	if len(protocols) < 1 {
		return "", false
	}
	for _, p := range s.passwords {
		if p == protocols[0] {
			return p, true
		}
	}
	return "", false
}

func (s *Server) StatusSocketHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var headers http.Header
	password, auth := s.isAuth(r)
	if auth {
		headers = http.Header{"Sec-WebSocket-Protocol": []string{password}}
	}

	clientName := r.FormValue("client")
	highres := r.FormValue("highres") != ""
	throttle := r.FormValue("throttle") != ""

	conn, err := upgrader.Upgrade(w, r, headers)
	if err != nil {
		log.Println(err)
		return
	}

	auth = auth || isLocal(r)

	log.Printf("New client %q from %q, highres: %v throttle: %v", clientName, r.RemoteAddr, highres, throttle)

	authClient := AuthorizedClient{
		RemoteAddr: r.RemoteAddr,
		Name:       clientName,
	}

	if auth {
		s.statusMu.Lock()
		s.status.AddAuthorizedClient(authClient)
		s.statusMu.Unlock()
	}

	t := &ThrottledTimer{period: 25 * time.Millisecond, throttle: throttle}
	t.cond = sync.NewCond(&t.mu)

	// Read and process incoming messages
	go func() {
		defer func() {
			s.statusMu.Lock()
			defer s.statusMu.Unlock()
			s.status.RemoveAuthorizedClient(authClient)
		}()
		for {
			var msg Command
			if err := conn.ReadJSON(&msg); err != nil {
				log.Printf("parsing json: %v", err)
				cancel()
				conn.Close()
				break
			}
			if msg.Command == "ack" {
				t.Ack(msg.SequenceNumber)
				continue
			}
			if !auth {
				log.Printf("Unauthenticated connection tried to %+v", msg)
				continue
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
			case "exit_shutdown":
				s.r.ExitShutdown()
			case "set_azimuth_offset":
				s.statusMu.Lock()
				s.status.OffsetAz = msg.Position
				s.statusMu.Unlock()
				s.r.SetAzimuthOffset(s.status.OffsetAz)
			case "set_elevation_offset":
				s.statusMu.Lock()
				s.status.OffsetEl = msg.Position
				s.statusMu.Unlock()
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
			case "set_band_tx":
				s.seq.SetBandTX(msg.Band, msg.Enabled)
			case "set_band_rx":
				// Cancel TX
				s.seq.SetBandTX(msg.Band, false)
				s.seq.SetBandRX(msg.Band, msg.Enabled)
			default:
				log.Printf("Unknown command: %+v", msg)
			}
			s.mu.Unlock()
		}
	}()

	seq := 0

	send := func(status Status) {
		status.SequenceNumber = seq
		seq++
		status.Authorized = auth
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
	status := s.status.Clone()
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
			status = s.status.Clone()
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
			if highres {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-t.Wait(seq):
			}
		}
	}
}

type ThrottledTimer struct {
	period   time.Duration
	throttle bool
	mu       sync.Mutex
	cond     *sync.Cond
	ack      int
}

func (t *ThrottledTimer) Ack(seq int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if seq > t.ack {
		t.ack = seq
		t.cond.Broadcast()
	}
}

func (t *ThrottledTimer) Wait(seq int) <-chan time.Time {
	if !t.throttle {
		return time.After(t.period)
	}
	c := make(chan time.Time, 1)
	go func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		for {
			if seq-t.ack < 5 {
				time.Sleep(t.period)
				c <- time.Now()
				return
			}
			t.cond.Wait()
		}
	}()
	return c
}

func (s *Server) statusCallback(status rci.Status) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.Status = status
	s.statusCond.Broadcast()
}

func (s *Server) sequencerStatusCallback(status sequencer.Status) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.Sequencer = status
	s.statusCond.Broadcast()
}
