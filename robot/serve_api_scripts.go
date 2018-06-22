package robot

import (
	"bytes"
	"encoding/json"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
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
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		if TypeScript(script.Type) != Tsend && TypeScript(script.Type) != Trespond {
			w.WriteHeader(http.StatusBadRequest)
			data, _ := json.Marshal(HttpError{
				Code:    http.StatusBadRequest,
				Message: "Invalid type was given, only 'send' and 'respond' type are allowed.",
			})
			w.Write(data)
			return
		}
		if g.isRemoteScriptExists(script) {
			existingScript = append(existingScript, script)
		}
		err = g.checkRemoteScript(script)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			data, _ := json.Marshal(HttpError{
				Code:    http.StatusBadRequest,
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
	for _, rmtScript := range tmpScripts {
		g.Store().Create(&rmtScript)
		g.RegisterScript(g.remoteScriptToScript(rmtScript))
		log.Info("Client '%s' on api registered: %s.", getRemoteIp(req), rmtScript.String())
	}

	w.WriteHeader(http.StatusCreated)
}
func (g *Gubot) remoteScriptToScript(rmtScript RemoteScript) Script {
	script := rmtScript.ToScript()
	script.Function = func(envelop Envelop, submatch [][]string) ([]string, error) {
		return g.sendEnvelopToScript(envelop, submatch, rmtScript)
	}
	return script
}
func (g *Gubot) deleteRemoteScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	tmpScripts, err := g.retrieveRemoteScript(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		w.Write(data)
		return
	}
	for _, script := range tmpScripts {
		if !g.isRemoteScriptExists(script) {
			continue
		}
		var whereScript RemoteScript
		whereScript.Name = script.Name
		g.Store().Unscoped().Where(&whereScript).Delete(RemoteScript{})
		g.UnregisterScript(g.remoteScriptToScript(script))
		log.Info("Client '%s' on api delete script: %s.", getRemoteIp(req), script.String())
	}
	w.WriteHeader(http.StatusOK)
}
func (g *Gubot) listRemoteScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)

	var rmtScripts []RemoteScript
	robot.Store().Find(&rmtScripts)
	data, _ := json.MarshalIndent(rmtScripts, "", "\t")
	w.Write(data)
}
func (g *Gubot) updateRemoteScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	notExistingScript := make([]RemoteScript, 0)
	tmpScripts, err := g.retrieveRemoteScript(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(HttpError{
			Code:    http.StatusBadRequest,
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
		var whereScript RemoteScript
		whereScript.Name = script.Name
		g.Store().Where(&whereScript).First(&dbScript)
		g.UnregisterScript(dbScript.ToScript())
		log.Infof("Client '%s' on api update script: %s.", getRemoteIp(req), dbScript.String())
		dbScript.Matcher = script.Matcher
		dbScript.Type = script.Type
		dbScript.Url = script.Url
		dbScript.TriggerOnMention = script.TriggerOnMention
		dbScript.Description = script.Description
		dbScript.Example = script.Example
		g.Store().Save(&dbScript)
		g.RegisterScript(g.remoteScriptToScript(script))
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
			Code:    http.StatusBadRequest,
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
			Code:    http.StatusBadRequest,
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
func (g *Gubot) checkRemoteScript(script RemoteScript) error {
	if script.Name != "" && script.Matcher != "" && script.Type != "" && script.Url != "" {
		return nil
	}
	return errors.New("Script must give a json with matcher, name, type and url key.")
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
	var whereScript RemoteScript
	whereScript.Name = script.Name
	g.Store().Where(&whereScript).First(&fScript)
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
