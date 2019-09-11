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
	addr          = flag.String("addr", "127.0.0.1:8502", "address to listen on")
	rotctldAddr   = flag.String("rotctld_addr", "127.0.0.1:4533", "address to listen for rotctld commands on")
	password      = flag.String("password", "", "password to require on remote connections")
	staticDir     = flag.String("static_dir", "static", "directory containing static files")
	serialPort    = flag.String("serial", "", "RCI serial port name")
	latitude      = flag.Float64("latitude", 42.360326, "latitude of antenna")
	longitude     = flag.Float64("longitude", -71.089324, "longitude of antenna")
	height        = flag.Float64("height", 100, "height of antenna (meters)")
	temperature   = flag.Float64("temperature", 15, "temperature (celsius)")
	pressure      = flag.Float64("pressure", 1010, "pressure (millibars)")
	azOffset      = flag.Float64("az_offset", 5.5, "azimuth offset (degrees)")
	elOffset      = flag.Float64("el_offset", -5.5, "elevation offset (degrees)")
	seqSerialPort = flag.String("sequencer_serial", "", "sequencer serial port name")
	seqURL        = flag.String("sequencer_url", "", "remote sequencer URL")
	seqBaud       = flag.Int("sequencer_baud", 19200, "sequencer baud rate")
)

func MaxAge(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=60, public, must-revalidate, proxy-revalidate")
		h.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	ctx := context.Background()
	place := novas.NewPlace(*latitude, *longitude, *height, *temperature, *pressure)
	server, err := NewServer(ctx, *serialPort, *password, *latitude, *longitude, place, *azOffset, *elOffset, *seqURL, *seqSerialPort, *seqBaud)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.ListenRotctld(ctx, *rotctldAddr); err != nil {
		log.Fatal(err)
	}
	r := mux.NewRouter()
	r.HandleFunc("/api/status", server.StatusHandler)
	r.HandleFunc("/api/ws", server.StatusSocketHandler)
	r.PathPrefix("/debug").Handler(http.DefaultServeMux)
	r.PathPrefix("/").Handler(MaxAge(http.FileServer(http.Dir(*staticDir))))
	srv := &http.Server{
		Handler:      r,
		Addr:         *addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Printf("Listening on %v", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
