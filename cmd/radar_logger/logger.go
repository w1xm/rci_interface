package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
)

func main() {
	// Create client
	server := os.Getenv("INFLUX_SERVER")
	if server == "" {
		server = "http://localhost:9999"
	}
	client := influxdb2.NewClient(server, os.Getenv("INFLUX_TOKEN"))
	defer client.Close()
	// Get non-blocking write client
	writeApi := client.WriteApi("w1xm", "radar.raw")
	defer writeApi.Close()
	// Get errors channel
	errorsCh := writeApi.Errors()
	// Create go proc for reading and logging errors
	go func() {
		for err := range errorsCh {
			log.Printf("write error: %v", err)
		}
	}()
	for {
		if err := logData(writeApi); err != nil {
			log.Print(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func flattenStatus(fields map[string]interface{}, status interface{}, prefix string) {
	switch status := status.(type) {
	case map[string]interface{}:
		for k, v := range status {
			flattenStatus(fields, v, prefix+"."+k)
		}
	case []interface{}:
		for k, v := range status {
			flattenStatus(fields, v, fmt.Sprintf("%s.%d", prefix, k))
		}
	default:
		fields[prefix[1:]] = status
	}
}

func logData(writeApi api.WriteApi) error {
	url := os.Getenv("RCI_ADDRESS")
	if url == "" {
		url = "ws://localhost:8502/api/ws"
	}
	defer writeApi.Flush()
	var dialer websocket.Dialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	for {
		var status interface{}
		if err := conn.ReadJSON(&status); err != nil {
			return err
			break
		}
		fields := make(map[string]interface{})
		flattenStatus(fields, status, "")

		p := influxdb2.NewPoint("radar.status",
			nil,
			fields,
			time.Now(),
		)
		// write asynchronously
		writeApi.WritePoint(p)
	}
	return nil
}
