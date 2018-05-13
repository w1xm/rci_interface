package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

var (
	staticDir = flag.String("static_dir", "static", "directory containing static files")
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Command struct {
	Register uint16 `json:"register"`
	Value    int16  `json:"value"`
}

type Status struct {
	Registers [12]int16
	Diag      int16
	// AzPos and ElPos are in decimal degrees.
	// They are calculated as 360*(reg/65536).
	AzPos double
	ElPos double
	// AzVel and ElVel are in RPM.
	// Positive indicates clockwise.
	// They are calculated as 60*(reg/65536).
	AzVel double
	ElVel double
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
}

func StatusSocket(w http.ResponseWriter, r *http.Request) {
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

	for {
		select {
		case <-ctx.Done():
			return
		}
		var status Status

		data, err := json.Marshal(controls)
		if err != nil {
			log.Print(err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Print(err)
			return
		}
	}
}

func main() {
	r := mux.NewRouter()
	r.Handle("/", http.FileServer(http.Dir(*staticDir)))
	r.Handle("/ws", http.HandlerFunc(StatusSocket))
	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8502",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
