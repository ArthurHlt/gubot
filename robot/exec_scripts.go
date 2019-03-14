package robot

import (
	"bytes"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

func (g *Gubot) registerProgramScripts(programs []ProgramScript) error {
	for _, p := range programs {
		script := p.ToScript()
		script.Function = func(envelop Envelop, submatch [][]string) ([]string, error) {
			return g.sendEnvelopToProgram(envelop, submatch, p)
		}
		err := g.RegisterScript(script)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) sendEnvelopToProgram(envelop Envelop, subMatch [][]string, script ProgramScript) ([]string, error) {
	dataToSend := struct {
		Envelop
		SubMatch [][]string `json:"sub_match"`
	}{envelop, subMatch}
	messages := make([]string, 0)
	jsonMessage, err := json.Marshal(dataToSend)
	if err != nil {
		return messages, err
	}
	cmd := exec.Command(script.Path, script.Args...)

	bufStdin := bytes.NewBuffer(jsonMessage)
	bufResp := &bytes.Buffer{}
	cmd.Stdout = bufResp
	cmd.Stderr = log.StandardLogger().Out
	cmd.Stdin = bufStdin
	cmd.Env = os.Environ()
	err = cmd.Run()
	if err != nil {
		return messages, err
	}

	err = json.NewDecoder(bufResp).Decode(&messages)
	return messages, err
}
