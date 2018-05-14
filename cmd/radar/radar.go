package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/gorilla/mux"
	"github.com/pebbe/novas"
)

var (
	staticDir   = flag.String("static_dir", "static", "directory containing static files")
	serialPort  = flag.String("serial", "", "serial port name")
	latitude    = flag.Float64("latitude", 42.360326, "latitude of antenna")
	longitude   = flag.Float64("longitude", -71.089324, "longitude of antenna")
	height      = flag.Float64("height", 100, "height of antenna (meters)")
	temperature = flag.Float64("temperature", 15, "temperature (celsius)")
	pressure    = flag.Float64("pressure", 1010, "pressure (millibars)")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	place := novas.NewPlace(*latitude, *longitude, *height, *temperature, *pressure)
	server, err := NewServer(ctx, *serialPort, place)
	if err != nil {
		log.Fatal(err)
	}
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
