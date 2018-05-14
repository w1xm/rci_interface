package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/w1xm/rci_interface/rci"
)

var (
	staticDir  = flag.String("static_dir", "static", "directory containing static files")
	serialPort = flag.String("serial", "", "serial port name")
)

type Command struct {
	Register uint16 `json:"register"`
	Value    int16  `json:"value"`
}

func main() {
	flag.Parse()
	ctx := context.Background()
	server := NewServer()
	ri, err := rci.Connect(ctx, *serialPort, server.statusCallback)
	if err != nil {
		log.Fatal(err)
	}
	server.r = ri
	r := mux.NewRouter()
	r.Handle("/", http.FileServer(http.Dir(*staticDir)))
	r.Handle("/api/status", http.HandlerFunc(server.StatusHandler))
	r.Handle("/api/ws", http.HandlerFunc(server.StatusSocketHandler))
	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8502",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Printf("Listening on %v", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
