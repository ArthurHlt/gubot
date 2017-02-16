package robot

import (
	"net/http"
	"net/url"
	"io/ioutil"
	"encoding/json"
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
	envelop := Envelop{}

	envelop.ChannelName = getParamByRegex("chan.*", params)
	envelop.IconUrl = getParamByRegex("(icon|pic|image|img).*", params)

	user := UserEnvelop{}
	user.ChannelName = getParamByRegex("chan.*", params)
	user.Name = getParamByRegex("(user_name|username|email|mail|user).*", params)
	user.Id = getParamByRegex("(user_id|userid|user_uuid|useruuid|email|mail|user).*", params)
	envelop.User = user
	message := getParamByRegex("(mess|msg|text).*", params)
	if message == "" {
		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		message = string(data)
	}
	envelop.Message = message
	g.Receive(envelop)
	w.WriteHeader(http.StatusOK)
}
func getParamByRegex(matcher string, params url.Values) string {
	for key, values := range params {
		if match("(?i)" + matcher, key) {
			return values[0]
		}
	}
	return ""
}
