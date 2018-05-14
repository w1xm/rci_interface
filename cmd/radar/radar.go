package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/gorilla/mux"
	"github.com/w1xm/rci_interface/rci"
)

var (
	staticDir  = flag.String("static_dir", "static", "directory containing static files")
	serialPort = flag.String("serial", "", "serial port name")
)

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
	r.Handle("/api/status", http.HandlerFunc(server.StatusHandler))
	r.Handle("/api/ws", http.HandlerFunc(server.StatusSocketHandler))
	r.PathPrefix("/debug").Handler(http.DefaultServeMux)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(*staticDir)))
	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8502",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Printf("Listening on %v", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
