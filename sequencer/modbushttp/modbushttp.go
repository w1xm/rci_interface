package modbushttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/goburrow/modbus"
)

type SendResponse struct {
	ADUResponse []byte
	Error       string
}

type Client struct {
	*modbus.RTUClientHandler

	baseURL string
}

func NewClient(baseURL string) *Client {
	handler := modbus.NewRTUClientHandler("/dev/null")
	handler.SlaveId = 1
	return &Client{
		RTUClientHandler: handler,
		baseURL:          baseURL,
	}
}

func (c *Client) Send(aduRequest []byte) ([]byte, error) {
	resp, err := http.Post(c.baseURL, "application/octet-stream", bytes.NewReader(aduRequest))
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status code: %s\n%s", resp.Status, string(body))
	}
	var sendResponse SendResponse
	if err := json.Unmarshal(body, &sendResponse); err != nil {
		return nil, err
	}
	if sendResponse.Error != "" {
		err = errors.New(sendResponse.Error)
	}
	return sendResponse.ADUResponse, err
}

func (c *Client) Connect() error {
	return nil
}

func (c *Client) Close() error {
	return nil
}
