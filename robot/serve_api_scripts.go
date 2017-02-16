package robot

import (
	"net/http"
	"encoding/json"
	"errors"
	"bytes"
	"strconv"
	"io"
	"io/ioutil"
)



type HttpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type TokenAuthHandler struct {
	h http.Handler
	g *Gubot
}
type EnvelopMessages struct {
	Envelop  Envelop  `json:"envelop"`
	Messages []string `json:"messages"`
}

func (g *Gubot) registerRemoteScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	existingScript := make([]RemoteScript, 0)
	tmpScripts, err := g.retrieveRemoteScript(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code: http.StatusBadRequest,
			Message: err.Error(),
		})
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		if TypeScript(script.Type) != Tsend && TypeScript(script.Type) != Trespond {
			w.WriteHeader(http.StatusBadRequest)
			data, _ := json.Marshal(HttpError{
				Code: http.StatusBadRequest,
				Message: "Invalid type was given, only 'send' and 'respond' type are allowed.",
			})
			w.Write(data)
			return
		}
		if g.isRemoteScriptExists(script) {
			existingScript = append(existingScript, script)
		}
		err = g.checkScript(script)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			data, _ := json.Marshal(HttpError{
				Code: http.StatusBadRequest,
				Message: err.Error(),
			})
			w.Write(data)
			return
		}
	}
	if len(existingScript) > 0 {
		w.WriteHeader(http.StatusConflict)
		data, _ := json.Marshal(existingScript)
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		g.Store().Create(&script)
	}
	w.WriteHeader(http.StatusCreated)
}
func (g *Gubot) deleteRemoteScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	tmpScripts, err := g.retrieveRemoteScript(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code: http.StatusBadRequest,
			Message: err.Error(),
		})
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		if !g.isRemoteScriptExists(script) {
			continue
		}
		g.Store().Unscoped().Where(&RemoteScript{Name: script.Name}).Delete(RemoteScript{})
	}
	w.WriteHeader(http.StatusOK)
}
func (g *Gubot) updateRemoteScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	notExistingScript := make([]RemoteScript, 0)
	tmpScripts, err := g.retrieveRemoteScript(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code: http.StatusBadRequest,
			Message: err.Error(),
		})
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		if !g.isRemoteScriptExists(script) {
			notExistingScript = append(notExistingScript, script)
		}

	}
	if len(notExistingScript) > 0 {
		w.WriteHeader(http.StatusNotModified)
		data, _ := json.Marshal(notExistingScript)
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		var dbScript RemoteScript
		g.Store().Where(&RemoteScript{Name: script.Name}).First(&dbScript)
		dbScript.Regex = script.Regex
		dbScript.Type = script.Type
		dbScript.Url = script.Url
		g.Store().Save(&dbScript)
	}
	w.WriteHeader(http.StatusOK)
}
func (g *Gubot) sendMessagesRemoteScripts(w http.ResponseWriter, req *http.Request) {
	g.envelopeMessagesSend(w, req, Tsend)
}
func (g *Gubot) respondMessagesRemoteScripts(w http.ResponseWriter, req *http.Request) {
	g.envelopeMessagesSend(w, req, Trespond)
}
func (g *Gubot) envelopeMessagesSend(w http.ResponseWriter, req *http.Request, typeScript TypeScript) {
	var envMessages EnvelopMessages
	err := json.NewDecoder(req.Body).Decode(&envMessages)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code: http.StatusBadRequest,
			Message: "Invalid json",
		})
		w.Write(data)
		return
	}
	if typeScript == Tsend {
		err = g.SendMessages(envMessages.Envelop, envMessages.Messages...)
	} else {
		err = g.RespondMessages(envMessages.Envelop, envMessages.Messages...)
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code: http.StatusBadRequest,
			Message: err.Error(),
		})
		w.Write(data)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (g *Gubot) retrieveRemoteScript(r io.Reader) ([]RemoteScript, error) {
	tmpScripts := make([]RemoteScript, 0)
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return tmpScripts, err
	}
	err = json.Unmarshal(b, &tmpScripts)
	if err == nil {
		return tmpScripts, nil
	}
	var tmpScript RemoteScript
	err = json.Unmarshal(b, &tmpScript)
	if err != nil {
		return tmpScripts, err
	}
	return []RemoteScript{tmpScript}, nil
}
func (g *Gubot) checkScript(script RemoteScript) error {
	if script.Name != "" && script.Regex != "" && script.Type != "" && script.Url != "" {
		return nil
	}
	return errors.New("Script must give a json with regex, name, type and url key.")
}
func (g *Gubot) sendToScript(envelop Envelop, typeScript TypeScript) ([]string, error) {
	messages := make([]string, 0)
	var remoteScripts []RemoteScript
	g.Store().Find(&remoteScripts)
	for _, script := range remoteScripts {
		if TypeScript(script.Type) != typeScript || !match(script.Regex, envelop.Message) {
			continue
		}
		tmpMessages, err := g.sendEnvelopToScript(envelop, allSubMatch(script.Regex, envelop.Message), script)
		if err != nil {
			return messages, err
		}
		messages = append(messages, tmpMessages...)
	}
	return messages, nil
}
func (g *Gubot) sendEnvelopToScript(envelop Envelop, subMatch [][]string, script RemoteScript) ([]string, error) {
	dataToSend := struct {
		Envelop
		SubMatch [][]string `json:"sub_match"`
	}{envelop, subMatch}
	messages := make([]string, 0)
	jsonMessage, err := json.Marshal(dataToSend)
	if err != nil {
		return messages, err
	}
	req, err := http.NewRequest("POST", script.Url, bytes.NewBuffer(jsonMessage))
	if err != nil {
		return messages, err
	}
	req.Header.Set("Content-type", "application/json")
	resp, err := g.HttpClient().Do(req)
	if err != nil {
		return messages, err
	}
	if resp.StatusCode != http.StatusOK {
		return messages, errors.New(strconv.Itoa(resp.StatusCode) + " " + resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(&messages)
	if err != nil {
		return messages, err
	}
	return messages, nil
}
func (g *Gubot) isRemoteScriptExists(script RemoteScript) bool {
	var fScript RemoteScript
	g.Store().Where(&RemoteScript{Name: script.Name}).First(&fScript)
	return fScript.Name != ""
}

func (t TokenAuthHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !t.g.IsSecured(w, req) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("401 unauthorized"))
		return
	}
	t.h.ServeHTTP(w, req)
}