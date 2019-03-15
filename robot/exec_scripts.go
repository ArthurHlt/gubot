package robot

import (
	"bytes"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
)

const (
	ProgramActionRegister TypeProgramAction = "register"
	ProgramActionReceive  TypeProgramAction = "receive"
)

type TypeProgramAction string

type ProgramDefinition struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Example          string `json:"example"`
	Matcher          string `json:"matcher"`
	TriggerOnMention bool   `json:"trigger_on_mention"`
}

type ProgramAction struct {
	Action TypeProgramAction `json:"action"`
	Data   interface{}       `json:"data,omitempty"`
}

func (d ProgramDefinition) ToScript() Script {
	return Script{
		Name:             d.Name,
		Type:             TypeScript(d.Type),
		Description:      d.Description,
		TriggerOnMention: d.TriggerOnMention,
		Matcher:          d.Matcher,
		Example:          d.Example,
	}
}

func (g *Gubot) registerProgramScripts(programs []ProgramScript) error {
	for _, p := range programs {
		err := g.registerProgramScript(p)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) registerProgramScript(program ProgramScript) error {
	bufResp, err := sendToProgram(program, ProgramAction{
		Action: ProgramActionRegister,
	})
	if err != nil {
		return err
	}
	scriptDefinitions := make([]ProgramDefinition, 0)
	err = json.NewDecoder(bufResp).Decode(&scriptDefinitions)
	if err != nil {
		return err
	}
	for _, sd := range scriptDefinitions {
		script := sd.ToScript()
		script.Function = func(envelop Envelop, submatch [][]string) ([]string, error) {
			return g.sendEnvelopToProgram(envelop, submatch, program)
		}
		err := g.RegisterScript(script)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) sendEnvelopToProgram(envelop Envelop, subMatch [][]string, program ProgramScript) ([]string, error) {
	dataToSend := struct {
		Envelop
		SubMatch [][]string `json:"sub_match"`
	}{envelop, subMatch}

	messages := make([]string, 0)

	bufResp, err := sendToProgram(program, ProgramAction{
		Action: ProgramActionReceive,
		Data:   dataToSend,
	})
	if err != nil {
		return messages, err
	}
	err = json.NewDecoder(bufResp).Decode(&messages)
	return messages, err
}

func sendToProgram(program ProgramScript, data interface{}) (reader io.Reader, err error) {
	jsonMessage, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(program.Path, program.Args...)

	bufStdin := bytes.NewBuffer(jsonMessage)
	bufResp := &bytes.Buffer{}
	cmd.Stdout = bufResp
	cmd.Stderr = log.StandardLogger().Out
	cmd.Stdin = bufStdin
	cmd.Env = os.Environ()
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	return bufResp, nil
}
