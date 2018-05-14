package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/w1xm/rci_interface/rci"
)

type Server struct {
	mu sync.Mutex
	r  *rci.RCI

	statusMu   sync.RWMutex
	statusCond *sync.Cond
	status     rci.Status
}

func NewServer() *Server {
	s := &Server{}
	s.statusCond = sync.NewCond(s.statusMu.RLocker())
	return s
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
				cancel()
				conn.Close()
				break
			}
			// Do thing
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

	for {
		select {
		case <-ctx.Done():
			return
		}
		s.statusMu.RLock()
		s.statusCond.Wait()
		status := s.status
		s.statusMu.RUnlock()
		send(status)
	}
}

func (s *Server) statusCallback(status rci.Status) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status = status
	s.statusCond.Broadcast()
}