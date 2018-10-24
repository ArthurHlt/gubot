package robot

import (
	"encoding/json"
	"github.com/jfrogdev/jfrog-cli-go/utils/cliutils/log"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func (g Gubot) showScripts(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	data, _ := json.MarshalIndent(g.scripts, "", "\t")
	w.Write(data)
}

func (g *Gubot) incoming(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	for keyParam, param := range req.Form {
		params[keyParam] = param
	}
	for keyParam, param := range req.PostForm {
		params[keyParam] = param
	}
	envelop := valuesToEnvelop(params)
	if envelop.Message == "" {
		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		envelop.Message = string(data)
	}
	g.Receive(*envelop)
	w.WriteHeader(http.StatusOK)
}

func (g *Gubot) slashCommand(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	for keyParam, param := range req.Form {
		params[keyParam] = param
	}
	for keyParam, param := range req.PostForm {
		params[keyParam] = param
	}

	token := getParamByRegex("token.*", params)
	if token == "" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Token empty or not found"))
		log.Error("Token empty or not found")
		return
	}
	var slashToken SlashCommandToken
	var c int
	err := g.store.Where("id = ?", token).Find(&slashToken).Count(&c).Error
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Error(err.Error())
	}
	if c == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Token empty or not found"))
		log.Error("Token empty or not found")
		return
	}

	envelop := valuesToEnvelop(params)
	envelop.ChannelName = strings.Split(envelop.ChannelName, "&")[0]

	result, err := g.DispatchCommand(slashToken, *envelop)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Error(err.Error())
	}
	if result == nil {
		return
	}
	w.Header().Add("Content-Type", "application/json")
	b, _ := json.Marshal(result)
	w.Write(b)
}

func valuesToEnvelop(params url.Values) *Envelop {
	envelop := &Envelop{
		Properties: make(map[string]interface{}),
	}

	envelop.ChannelName = getParamByRegex("chan.*", params)
	envelop.IconUrl = getParamByRegex("(icon|pic|image|img).*", params)

	user := UserEnvelop{}
	user.ChannelName = getParamByRegex("chan.*", params)
	user.Name = getParamByRegex("(user_name|username).*", params)
	if user.Name == "" {
		user.Name = getParamByRegex("(email|mail).*", params)
	}
	if user.Name == "" {
		user.Name = getParamByRegex("user.*", params)
	}

	user.Id = getParamByRegex("(user_id|userid|user_uuid|useruuid).*", params)
	if user.Id == "" {
		user.Id = getParamByRegex("(email|mail).*", params)
	}
	if user.Id == "" {
		user.Id = getParamByRegex("user.*", params)
	}

	envelop.User = user
	envelop.Message = getParamByRegex("(mess|msg|text).*", params)
	envelop.Properties = valuesToMap(params)
	return envelop
}

func getParamByRegex(matcher string, params url.Values) string {
	for key, values := range params {
		if match("(?i)"+matcher, key) {
			return values[0]
		}
	}
	return ""
}

func valuesToMap(params url.Values) map[string]interface{} {
	finalMap := make(map[string]interface{})
	for key, values := range params {
		finalMap[key] = strings.Join(values, ",")
	}
	return finalMap
}
