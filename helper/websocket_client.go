package helper

import (
	"github.com/gorilla/websocket"
	"github.com/ArthurHlt/gubot/robot"
	"encoding/json"
	"fmt"
	"errors"
	"crypto/tls"
	"net/http"
)

type WebSocketEvent struct {
	Event robot.GubotEvent
	Error error
}
type WebSocketClient struct {
	Url                string          // The location of the server like "ws://localhost:8065"
	ApiUrl             string          // The api location of the server like "ws://localhost:8065/api"
	Conn               *websocket.Conn // The WebSocket connection
	AuthToken          string          // The token used to open the WebSocket
	Sequence           int             // The ever-incrementing sequence attached to each WebSocket action
	EventChannel       chan *WebSocketEvent
	InsecureSkipVerify bool
	ListenError        error
}

func NewWebSocketClient(gubotUrl, token string) *WebSocketClient {
	return &WebSocketClient{
		Url: gubotUrl,
		ApiUrl: gubotUrl + "/api",
		Conn: nil,
		AuthToken: token,
		Sequence: 1,
		EventChannel: make(chan *WebSocketEvent, 100),
		InsecureSkipVerify: false,
		ListenError: nil,
	}
}
func (wsc *WebSocketClient) Connect() error {
	var err error
	dialer := &websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: wsc.InsecureSkipVerify,
		},
	}
	wsc.Conn, _, err = dialer.Dial(wsc.ApiUrl + "/websocket", nil)
	if err != nil {
		return err
	}

	wsc.EventChannel = make(chan *WebSocketEvent, 100)
	wsc.SendToken()

	return nil
}
func (wsc *WebSocketClient) Listen() {
	go func() {
		defer func() {
			wsc.Conn.Close()
			close(wsc.EventChannel)
		}()
		for {
			var rawMsg json.RawMessage
			var err error
			if _, rawMsg, err = wsc.Conn.ReadMessage(); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					wsc.ListenError = errors.New(fmt.Sprintf("Could not connect %s", err.Error()))
				}

				return
			}

			var r robot.WebSocketRequest
			if err := json.Unmarshal(rawMsg, &r); err == nil && r.IsValid() {
				var eventError error
				if r.IsInError() {
					eventError = errors.New(r.Error)
				}
				wsc.EventChannel <- &WebSocketEvent{
					Event: r.Event,
					Error: eventError,
				}
				wsc.SendAck(r)
				continue
			}

		}
	}()
}
func (wsc *WebSocketClient) SendToken() {
	req := &robot.WebSocketTokenRequest{}
	req.Seq = wsc.Sequence
	req.Token = wsc.AuthToken

	wsc.Sequence++

	wsc.Conn.WriteJSON(req)
}
func (wsc *WebSocketClient) SendNack(r robot.WebSocketRequest) {
	req := &robot.WebSocketRequest{}
	req.SeqReply = r.Seq
	req.Status = robot.WEB_SOCKET_STATUS_FAIL

	wsc.Conn.WriteJSON(req)
}
func (wsc *WebSocketClient) SendAck(r robot.WebSocketRequest) {
	req := &robot.WebSocketRequest{}
	req.SeqReply = r.Seq
	req.Status = robot.WEB_SOCKET_STATUS_OK

	wsc.Sequence = req.Seq

	wsc.Conn.WriteJSON(req)
}
func (wsc *WebSocketClient) Close() {
	wsc.Conn.Close()
}