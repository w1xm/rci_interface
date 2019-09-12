package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/goburrow/modbus"
	"github.com/gorilla/mux"
	"github.com/w1xm/rci_interface/sequencer/modbushttp"
)

var (
	addr          = flag.String("addr", "127.0.0.1:8502", "address to listen on")
	password      = flag.String("password", "", "password to require on remote connections")
	seqSerialPort = flag.String("sequencer_serial", "", "sequencer serial port name")
	seqBaud       = flag.Int("sequencer_baud", 19200, "sequencer baud rate")
)

type Server struct {
	handler  *modbus.RTUClientHandler
	password string
}

func NewServer(ctx context.Context, port string, baud int, password string) *Server {
	handler := modbus.NewRTUClientHandler(port)
	handler.BaudRate = baud
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	handler.Timeout = 1 * time.Second
	handler.SlaveId = 1
	return &Server{
		handler: handler,
		password: password,
	}
}

func (s *Server) SendHandler(w http.ResponseWriter, r *http.Request) {
	_, pass, ok := r.BasicAuth()
	if !ok || pass != s.password {
		http.Error(w, "wrong password", http.StatusUnauthorized)
		return
	}
	//Send(aduRequest []byte) (aduResponse []byte, err error) {
	err := func() error {
		aduRequest, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}
		aduResponse, err := s.handler.Send(aduRequest)
		var errString string
		if err != nil {
			errString = err.Error()
		}
		if body, err := json.Marshal(&modbushttp.SendResponse{
			ADUResponse: aduResponse,
			Error:       errString,
		}); err != nil {
			return err
		} else {
			_, err := w.Write(body)
			return err
		}
	}()
	if err != nil {
		log.Printf("SendHandler: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
}

func main() {
	flag.Parse()
	ctx := context.Background()
	server := NewServer(ctx, *seqSerialPort, *seqBaud, *password)
	r := mux.NewRouter()
	r.Handle("/api/send", http.HandlerFunc(server.SendHandler))
	r.PathPrefix("/debug").Handler(http.DefaultServeMux)
	srv := &http.Server{
		Handler:      r,
		Addr:         *addr,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	log.Printf("Listening on %v", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
