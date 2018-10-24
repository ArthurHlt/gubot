package mattermost_user

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ArthurHlt/gubot/robot"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-multierror"
	"github.com/mattermost/mattermost-server/model"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func init() {
	robot.RegisterAdapter(NewMattermostUserAdapter())
}

type PostData struct {
	ID            string    `json:"id"`
	CreateAt      int64     `json:"create_at"`
	UpdateAt      int64     `json:"update_at"`
	DeleteAt      int       `json:"delete_at"`
	UserID        string    `json:"user_id"`
	ChannelID     string    `json:"channel_id"`
	RootID        string    `json:"root_id"`
	ParentID      string    `json:"parent_id"`
	OriginalID    string    `json:"original_id"`
	Message       string    `json:"message"`
	Type          string    `json:"type"`
	Props         PropsData `json:"props"`
	Hashtags      string    `json:"hashtags"`
	PendingPostID string    `json:"pending_post_id"`
}

type PropsData struct {
	FromWebhook      string `json:"from_webhook"`
	OverrideUsername string `json:"override_username"`
}

type MattermostUserConfig struct {
	MattermostUsername            string
	MattermostUserID              string
	MattermostPassword            string
	MattermostApi                 string
	MattermostChannels            []string
	MattermostTickStatusInSeconds int `cloud-default:"30"`
}

type MattermostUserAdapter struct {
	clientWs    *model.WebSocketClient
	client      *model.Client4
	gubot       *robot.Gubot
	mutex       *sync.Mutex
	onlineUsers map[string]interface{}
	me          *model.User
}

func NewMattermostUserAdapter() robot.Adapter {
	return &MattermostUserAdapter{
		onlineUsers: make(map[string]interface{}),
		mutex:       new(sync.Mutex),
	}
}

func (a MattermostUserAdapter) Send(envelop robot.Envelop, message string) error {
	if envelop.ChannelName == "" && envelop.ChannelId == "" {
		return errors.New("You must provide a channel name or channel id in envelop")
	}
	var err error
	channelId := envelop.ChannelId
	if channelId == "" {
		_, channelId, err = a.getTeamIdAndChannelIdByChannelName(envelop.ChannelName)
		if err != nil {
			return err
		}
	}
	_, resp := a.client.CreatePost(&model.Post{
		ChannelId: channelId,
		Message:   message,
	})
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

func (a MattermostUserAdapter) SendDirect(envelop robot.Envelop, message string) error {
	channel, resp := a.client.CreateDirectChannel(a.me.Id, envelop.User.Id)
	if resp.Error != nil {
		return resp.Error
	}
	_, resp = a.client.CreatePost(&model.Post{
		ChannelId: channel.Id,
		Message:   message,
	})
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

func (a MattermostUserAdapter) getTeamIdByChannelId(channelId string) (string, error) {
	channel, resp := a.client.GetChannel(channelId, "")
	if resp.Error != nil {
		return "", resp.Error
	}
	return channel.TeamId, nil
}

func (a MattermostUserAdapter) getTeamIdAndChannelIdByChannelName(channelName string) (string, string, error) {
	teams, resp := a.client.GetAllTeams("", 0, 120)
	if resp.Error != nil {
		return "", "", resp.Error
	}
	for _, team := range teams {
		channel, appErr := a.GetChannelByName(team.Id, channelName)
		if appErr != nil || channel.Name != channelName {
			continue
		}
		return channel.TeamId, channel.Id, nil
	}
	return "", "", errors.New("Team id not found")
}

func (a MattermostUserAdapter) GetChannelById(teamId, channelId string) (*model.Channel, *model.AppError) {
	route := fmt.Sprintf("/teams/%v/channels/%v/", teamId, channelId)
	if r, err := a.client.DoApiGet(route, ""); err != nil {
		return nil, err
	} else {
		defer closeBody(r)
		return model.ChannelFromJson(r.Body), nil
	}
}

func (a MattermostUserAdapter) GetChannelByName(teamId, channelName string) (*model.Channel, *model.AppError) {
	route := fmt.Sprintf("/teams/%v/channels/name/%v", teamId, channelName)
	if r, err := a.client.DoApiGet(route, ""); err != nil {
		return nil, err
	} else {
		defer closeBody(r)
		return model.ChannelFromJson(r.Body), nil
	}
}

func closeBody(r *http.Response) {
	if r.Body != nil {
		ioutil.ReadAll(r.Body)
		r.Body.Close()
	}
}

func (a MattermostUserAdapter) Reply(envelop robot.Envelop, message string) error {
	return a.Send(envelop, "@"+envelop.User.Name+": "+message)
}

func (a *MattermostUserAdapter) Run(config interface{}, gubot *robot.Gubot) error {
	conf := config.(*MattermostUserConfig)
	a.gubot = gubot
	if conf.MattermostUsername == "" && conf.MattermostUserID == "" {
		return errors.New("mattermost_username or mattermost_user_id config param is required")
	}
	if conf.MattermostPassword == "" {
		return errors.New("mattermost_password config param is required")
	}
	if conf.MattermostApi == "" {
		return errors.New("mattermost_api config param is required")
	}

	mattApi, err := url.Parse(conf.MattermostApi)
	if err != nil {
		return err
	}
	wsMattApi, _ := url.Parse(conf.MattermostApi)

	if mattApi.Scheme == "https" || mattApi.Scheme == "wss" {
		mattApi.Scheme = "https"
		wsMattApi.Scheme = "wss"
	} else {
		mattApi.Scheme = "http"
		wsMattApi.Scheme = "ws"
	}
	client := model.NewAPIv4Client(mattApi.String())
	client.HttpClient = gubot.HttpClient()
	var resp *model.Response
	if conf.MattermostUsername != "" {
		_, resp = client.Login(conf.MattermostUsername, conf.MattermostPassword)
	} else {
		_, resp = client.LoginById(conf.MattermostUserID, conf.MattermostPassword)
	}

	if resp.Error != nil {
		return resp.Error
	}
	a.client = client
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: gubot.HttpClient().Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify,
	}
	clientWs, appErr := model.NewWebSocketClient(wsMattApi.String(), client.AuthToken)
	if appErr != nil {
		return appErr
	}
	a.clientWs = clientWs

	mattMe, resp := client.GetMe("")
	if resp.Error != nil {
		return resp.Error
	}
	a.me = mattMe
	clientWs.Listen()
	go func() {
		for {
			event := <-a.clientWs.EventChannel
			if event == nil {
				appErr := clientWs.Connect()
				if appErr != nil {
					log.Error("Error when reconnecting to web socket: " + appErr.Error())
				}
				clientWs.Listen()
				continue
			}
			if event.Event == model.WEBSOCKET_EVENT_USER_ADDED {
				a.gubot.Emit(robot.GubotEvent{
					Name:    robot.EVENT_ROBOT_CHANNEL_ENTER,
					Envelop: a.eventToEnvelop(event),
				})
				continue
			}
			if event.Event == model.WEBSOCKET_EVENT_USER_REMOVED {
				a.gubot.Emit(robot.GubotEvent{
					Name:    robot.EVENT_ROBOT_CHANNEL_LEAVE,
					Envelop: a.eventToEnvelop(event),
				})
				continue
			}
			if event.Event != model.WEBSOCKET_EVENT_POSTED {
				continue
			}
			channelName := event.Data["channel_name"].(string)
			if len(conf.MattermostChannels) > 0 {
				found := false
				for _, tmpChannel := range conf.MattermostChannels {
					if tmpChannel == channelName {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			a.sendingEnvelop(event, mattMe)
		}
	}()
	go func() {
		if conf.MattermostTickStatusInSeconds <= 0 {
			return
		}
		for {
			a.emitStatusChange()
			time.Sleep(time.Duration(conf.MattermostTickStatusInSeconds) * time.Second)
		}
	}()
	return nil
}

func (a *MattermostUserAdapter) emitStatusChange() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.clientWs.GetStatuses()
	resp := <-a.clientWs.ResponseChannel
	if resp == nil {
		return
	}
	onlineUsersReceived := resp.Data
	for userId, _ := range onlineUsersReceived {
		if _, ok := a.onlineUsers[userId]; !ok && userId != a.me.Id {
			a.gubot.Emit(robot.GubotEvent{
				Name:    robot.EVENT_ROBOT_USER_ONLINE,
				Envelop: a.userIdToDirectEnvelop(userId),
			})
		}
	}
	for userId, _ := range a.onlineUsers {
		if _, ok := onlineUsersReceived[userId]; !ok && userId != a.me.Id {
			a.gubot.Emit(robot.GubotEvent{
				Name:    robot.EVENT_ROBOT_USER_OFFLINE,
				Envelop: a.userIdToDirectEnvelop(userId),
			})
		}
	}
	a.onlineUsers = onlineUsersReceived
}

func (a MattermostUserAdapter) userIdToDirectEnvelop(userId string) robot.Envelop {
	user, resp := a.client.GetUser(userId, "")
	if resp.Error != nil {
		log.Error("Cannot transform event in envelop")
		return robot.Envelop{}
	}
	userEnvelop := robot.UserEnvelop{}
	envelop := robot.Envelop{}
	userName := user.Username
	channelName := a.me.Id + "__" + userId

	userEnvelop.Name = userName
	userEnvelop.ChannelName = channelName
	userEnvelop.Id = userId

	envelop.Properties = make(map[string]interface{})

	envelop.ChannelName = channelName
	envelop.User = userEnvelop
	return envelop
}

func (a MattermostUserAdapter) eventToEnvelop(event *model.WebSocketEvent) robot.Envelop {
	userId := event.Data["user_id"].(string)
	channelId := event.Broadcast.ChannelId
	user, resp := a.client.GetUser(userId, "")
	if resp.Error != nil {
		log.Error("Cannot transform event in envelop")
		return robot.Envelop{}
	}
	userEnvelop := robot.UserEnvelop{}
	envelop := robot.Envelop{}
	userName := user.Username

	userEnvelop.Name = userName
	userEnvelop.ChannelId = channelId
	userEnvelop.Id = userId

	envelop.Properties = make(map[string]interface{})
	if _, ok := event.Data["team_id"]; ok && event.Data["team_id"].(string) != "" {
		envelop.Properties["team_id"] = event.Data["team_id"].(string)
	}
	envelop.ChannelId = channelId
	envelop.User = userEnvelop
	return envelop
}

func (a MattermostUserAdapter) sendingEnvelop(event *model.WebSocketEvent, mattMe *model.User) {

	channelName := event.Data["channel_name"].(string)
	var postData PostData
	postDataRaw := event.Data["post"].(string)
	err := json.Unmarshal([]byte(postDataRaw), &postData)
	if err != nil {
		log.Error("Error when unmarshalling data: " + err.Error())
		return
	}
	if mattMe.Id == postData.UserID {
		return
	}
	// we don't talk with other bots
	if postData.Props.FromWebhook == "true" {
		return
	}
	user := robot.UserEnvelop{}
	envelop := robot.Envelop{}
	channelId := postData.ChannelID
	userName := event.Data["sender_name"].(string)

	user.Name = userName
	user.ChannelId = channelId
	user.ChannelName = channelName
	user.Id = postData.UserID

	envelop.Properties = make(map[string]interface{})
	envelop.Properties["team_id"] = event.Data["team_id"].(string)
	envelop.ChannelId = channelId
	envelop.ChannelName = channelName
	envelop.Message = postData.Message
	envelop.User = user
	mentioned := a.isMentioned(event)
	envelop.NotMentioned = !mentioned

	mentionKeys := []string{
		a.me.Nickname,
		a.me.Username,
		a.me.FirstName,
	}
	if rawMenKeys, ok := a.me.NotifyProps["mention_keys"]; ok {
		mentionKeys = append(mentionKeys, strings.Split(rawMenKeys, ",")...)
	}

	if mentioned {
		for _, mentionKey := range mentionKeys {
			envelop.Message = strings.Replace(envelop.Message, "@"+mentionKey, "", -1)
			envelop.Message = strings.Replace(envelop.Message, mentionKey, "", -1)
		}
		envelop.Message = strings.TrimSpace(envelop.Message)
	}
	a.gubot.Receive(envelop)
}

func (a MattermostUserAdapter) isMentioned(event *model.WebSocketEvent) bool {
	if event.Data["channel_type"].(string) == "D" {
		return true
	}
	if _, ok := event.Data["mentions"]; !ok {
		return false
	}
	mentionString := event.Data["mentions"].(string)
	var mentions []string
	err := json.Unmarshal([]byte(mentionString), &mentions)
	if err != nil {
		return false
	}
	for _, mention := range mentions {
		if mention == a.me.Id {
			return true
		}
	}
	return false
}

func (a MattermostUserAdapter) Name() string {
	return "mattermost user"
}

func (a MattermostUserAdapter) Config() interface{} {
	return MattermostUserConfig{}
}

func (a MattermostUserAdapter) Format(message string) (interface{}, error) {
	if message == "" {
		return nil, nil
	}
	return map[string]string{"text": message}, nil
}

func (a MattermostUserAdapter) Register(slashCommand robot.SlashCommand) ([]robot.SlashCommandToken, error) {
	teams, resp := a.client.GetTeamMembersForUser(a.me.Id, "")
	slashTokens := make([]robot.SlashCommandToken, 0)
	if resp.Error != nil {
		return slashTokens, resp.Error
	}
	var result error
	for _, team := range teams {
		slashToken, err := a.registerByTeam(team, slashCommand)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
		if slashToken.ID == "" {
			continue
		}
		slashTokens = append(slashTokens, slashToken)
	}
	return slashTokens, result
}

func (a MattermostUserAdapter) registerByTeam(team *model.TeamMember, slashCommand robot.SlashCommand) (robot.SlashCommandToken, error) {
	cmds, resp := a.client.ListCommands(team.TeamId, true)
	if resp.Error != nil {
		return robot.SlashCommandToken{}, resp.Error
	}
	for _, cmd := range cmds {
		if cmd.Trigger == slashCommand.Trigger {
			return robot.SlashCommandToken{}, nil
		}
	}

	mattCmd, resp := a.client.CreateCommand(&model.Command{
		TeamId:       team.TeamId,
		URL:          robot.SlashCommandUrl(),
		Method:       model.COMMAND_METHOD_POST,
		Trigger:      slashCommand.Trigger,
		AutoComplete: true,
		Description:  slashCommand.Description,
		DisplayName:  a.me.Username,
		IconURL:      robot.IconUrl(),
		Username:     a.me.Username,
	})
	if resp.Error != nil {
		return robot.SlashCommandToken{}, resp.Error
	}
	return robot.SlashCommandToken{
		AdapterName: a.Name(),
		CommandName: slashCommand.Trigger,
		ID:          mattCmd.Token,
	}, nil
}
