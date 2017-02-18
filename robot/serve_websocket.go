package robot

import (
	"net/http"
	"github.com/gorilla/websocket"
	"log"
	"fmt"
	"errors"
	"time"
)

const (
	WEB_SOCKET_MAX_MESSAGE_SIZE_KB int = 4096
	WEB_SOCKET_STATUS_OK = "OK"
	WEB_SOCKET_STATUS_FAIL = "FAIL"
	WEB_SOCKET_MAX_RETRY int = 3
	WEB_SOCKET_READ_DEADLINE int = 3
)

type WebSocketTokenRequest struct {
	WebSocketRequest
	Token string `json:"token"`
}
type WebSocketRequest struct {
	Event    GubotEvent `json:"event,omitempty"`
	Status   string     `json:"status"`
	Error    string     `json:"error,omitempty"`
	Seq      int        `json:"seq,omitempty"`
	SeqReply int        `json:"seq_reply,omitempty"`
}

func (g *Gubot) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  WEB_SOCKET_MAX_MESSAGE_SIZE_KB,
		WriteBufferSize: WEB_SOCKET_MAX_MESSAGE_SIZE_KB,
		CheckOrigin:     nil,
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.logger.Error("Upgrade:", err)
		return
	}
	g.logger.Info("Client '%s' on websocket trying to connect", getRemoteIp(r))
	defer func() {
		ws.Close()
		g.logger.Info("Client '%s' on websocket disconnected", getRemoteIp(r))
	}()
	seq := 1
	var tokenRequest WebSocketTokenRequest
	err = ws.ReadJSON(&tokenRequest)
	if err != nil {
		if websocket.IsCloseError(err) {
			return
		}
		ws.WriteJSON(WebSocketRequest{
			SeqReply: seq,
			Status: WEB_SOCKET_STATUS_FAIL,
			Error: err.Error(),
		})
		return
	}
	if !g.IsValidToken(tokenRequest.Token) {
		ws.WriteJSON(WebSocketRequest{
			SeqReply: seq,
			Status: WEB_SOCKET_STATUS_FAIL,
			Error: "Invalid token",
		})
		g.logger.Info("Client '%s' on websocket use wrong token", getRemoteIp(r))
		return
	}
	if tokenRequest.Seq != seq {
		ws.WriteJSON(WebSocketRequest{
			SeqReply: seq,
			Status: WEB_SOCKET_STATUS_FAIL,
			Error: fmt.Sprintf("Invalid seq receive, expected %d got %d", seq, tokenRequest.Seq),
		})
		return
	}
	g.logger.Info("Client '%s' on websocket is connected", getRemoteIp(r))
	err = ws.WriteJSON(WebSocketRequest{
		SeqReply: seq,
		Status: WEB_SOCKET_STATUS_OK,
	})
	if err != nil {
		if websocket.IsCloseError(err) {
			return
		}
		g.logger.Error("Error when writing ok status after received token.")
		return
	}
	seq ++
	for event := range g.On("*") {
		gubotEvent := ToGubotEvent(event)
		err = sendWebSocketEvent(ws, gubotEvent, seq)
		if err != nil {
			if websocket.IsCloseError(err) {
				return
			}
			log.Println(err)
			return
		}
		seq++
	}
}
func sendWebSocketEvent(ws *websocket.Conn, gubotEvent GubotEvent, seq int) error {
	for i := 0; i < WEB_SOCKET_MAX_RETRY; i++ {
		err := ws.WriteJSON(WebSocketRequest{
			Event: gubotEvent,
			Seq: seq,
			Status: WEB_SOCKET_STATUS_OK,
		})
		if err != nil {
			if websocket.IsCloseError(err) {
				return err
			}
			return errors.New(fmt.Sprintf("Error when writing event %s: %s .", gubotEvent.Action, err.Error()))
		}
		ws.SetReadDeadline(time.Now().Add(time.Duration(WEB_SOCKET_READ_DEADLINE) * time.Second))
		var resp WebSocketRequest
		err = ws.ReadJSON(&resp)
		if err != nil {
			if websocket.IsCloseError(err) {
				return err
			}
			return errors.New(fmt.Sprintf("Error when reading reply after event %s: %s .", gubotEvent.Action, err.Error()))

		}
		if resp.SeqReply != seq {
			ws.WriteJSON(WebSocketRequest{
				SeqReply: seq,
				Status: WEB_SOCKET_STATUS_FAIL,
				Error: fmt.Sprintf("Invalid seq_reply receive, expected %d got %d", seq, resp.Seq),
			})
			return nil
		}
		if resp.Status == WEB_SOCKET_STATUS_FAIL {
			continue
		}
		break
	}
	return nil
}
func (w WebSocketRequest) IsInError() bool {
	return w.Status == WEB_SOCKET_STATUS_FAIL && w.Error != ""
}
func (w WebSocketRequest) IsValid() bool {
	return w.Error != "" || w.Event.Action != ""
}
