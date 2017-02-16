package slack

import (
	"github.com/ArthurHlt/gubot/robot"
	"errors"
	"net/http"
	"log"
	"strings"
	"encoding/json"
	"bytes"
	"strconv"
)

func init() {
	robot.RegisterAdapter(NewSlackAdapter())
}

type SlackConfig struct {
	SlackTokens        []string
	SlackChannel       string
	SlackEndpoint      string
	SlackIconEmoji     string
	SlackIncomeUrl     string
	SlackIconUrl       string
	SlackGubotUsername string
}
type Notification struct {
	Text      string      `json:"text"`
	Username  string      `json:"username"`
	IconURL   interface{} `json:"icon_url,omitempty"`
	IconEmoji interface{} `json:"icon_emoji,omitempty"`
	Channel   interface{} `json:"channel"`
}
type SlackAdapter struct {
	config *SlackConfig
	gubot  *robot.Gubot
}

func NewSlackAdapter() robot.Adapter {
	return &SlackAdapter{}
}
func (a SlackAdapter) Send(envelop robot.Envelop, message string) error {
	notif := a.envelopToNotif(envelop)
	notif.Text = message
	jsonMessage, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", a.config.SlackIncomeUrl, bytes.NewBuffer(jsonMessage))
	if err != nil {
		return err
	}
	req.Header.Set("Content-type", "application/json")
	resp, err := robot.HttpClient().Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(strconv.Itoa(resp.StatusCode) + " " + resp.Status)
	}

	return nil
}
func (a SlackAdapter) envelopToNotif(envelop robot.Envelop) Notification {
	username := robot.Name()
	if a.config.SlackGubotUsername != "" {
		username = a.config.SlackGubotUsername
	}
	iconUrl := a.config.SlackIconUrl
	if iconUrl == "" {
		iconUrl = envelop.IconUrl
	}
	return Notification{
		Username: username,
		Channel: a.getChannel(envelop),
		IconEmoji: a.config.SlackIconEmoji,
		IconURL: iconUrl,
	}
}
func (a SlackAdapter) Reply(envelop robot.Envelop, message string) error {
	return a.Send(envelop, "@" + envelop.User.Name + ": " + message)
}
func (a SlackAdapter) getChannel(envelop robot.Envelop) string {
	if a.config.SlackChannel == "" {
		return envelop.ChannelName
	}
	return a.config.SlackChannel
}
func (a *SlackAdapter) Run(config interface{}, gubot *robot.Gubot) error {
	slackConf := config.(*SlackConfig)
	a.gubot = gubot
	if slackConf.SlackGubotUsername == "" {
		slackConf.SlackGubotUsername = gubot.Name()
	}
	if len(slackConf.SlackTokens) == 0 {
		slackConf.SlackTokens = gubot.Tokens()
	}
	if slackConf.SlackEndpoint == "" {
		slackConf.SlackEndpoint = "/slack"
	}
	if slackConf.SlackIncomeUrl == "" {
		return errors.New("slack_income_url config param is required")
	}
	a.config = slackConf
	gubot.Router().HandleFunc(a.config.SlackEndpoint, a.handler).Methods("POST")
	return nil
}
func (a SlackAdapter) handler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	if !a.isValidToken(req.PostForm.Get("token")) {
		w.WriteHeader(http.StatusUnauthorized)
		log.Println("Error with slack adapter: given token is not valid.")
		return
	}
	triggerWord := req.PostForm.Get("trigger_word")
	user := robot.UserEnvelop{}
	envelop := robot.Envelop{}
	envelop.Message = strings.TrimSpace(strings.TrimPrefix(req.PostForm.Get("text"), triggerWord))
	channel := req.PostForm.Get("channel_name")
	channelId := req.PostForm.Get("channel_id")
	envelop.ChannelName = channel
	envelop.ChannelId = channelId

	user.ChannelName = channel
	user.Id = req.PostForm.Get("user_id")
	user.Name = req.PostForm.Get("user_name")
	envelop.User = user

	a.gubot.Receive(envelop)
	w.WriteHeader(http.StatusOK)
}
func (a SlackAdapter) isValidToken(tokenToCheck string) bool {
	for _, token := range a.config.SlackTokens {
		if tokenToCheck == token {
			return true
		}
	}
	return false
}
func (a SlackAdapter) Name() string {
	return "slack"
}
func (a SlackAdapter) Config() interface{} {
	return SlackConfig{}
}
