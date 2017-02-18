package mattermost_user

import (
	"github.com/ArthurHlt/gubot/robot"
	"errors"
	"github.com/mattermost/platform/model"
	"net/url"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
	"strings"
	"github.com/ArthurHlt/gominlog"
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
	MattermostPassword            string
	MattermostApi                 string
	MattermostChannels            []string
	MattermostTickStatusInSeconds int `cloud:",default=30"`
}
type MattermostUserAdapter struct {
	clientWs    *model.WebSocketClient
	client      *model.Client
	gubot       *robot.Gubot
	mutex       *sync.Mutex
	logger      *gominlog.MinLog
	onlineUsers map[string]interface{}
	me          *model.User
}

func NewMattermostUserAdapter() robot.Adapter {
	return &MattermostUserAdapter{
		onlineUsers: make(map[string]interface{}),
		mutex: new(sync.Mutex),
	}
}
func (a MattermostUserAdapter) Send(envelop robot.Envelop, message string) error {
	if envelop.ChannelName == "" && envelop.ChannelId == "" {
		return errors.New("You must provide a channel name or channel id in envelop")
	}
	var teamId string
	var err error
	channelId := envelop.ChannelId
	if channelId == "" {
		teamId, channelId, err = a.getTeamIdAndChannelIdByChannelName(envelop.ChannelName)
		if err != nil {
			return err
		}
	}
	if _, ok := envelop.Properties["team_id"]; ok && envelop.Properties["team_id"].(string) != "" {
		teamId = envelop.Properties["team_id"].(string)
	}
	if teamId == "" {
		teamId, err = a.getTeamIdByChannelId(channelId)
		if err != nil {
			return err
		}
	}
	post := &model.Post{
		ChannelId: channelId,
		Message: message,
	}
	route := fmt.Sprintf("/teams/%v/channels/%v", teamId, channelId)
	if r, err := a.client.DoApiPost(route + "/posts/create", post.ToJson()); err != nil {
		return err
	} else {
		closeBody(r)
	}

	return nil
}
func (a MattermostUserAdapter) getTeamIdByChannelId(channelId string) (string, error) {
	result, appErr := a.client.GetAllTeams()
	if appErr != nil {
		return "", appErr
	}
	teams := result.Data.(map[string]*model.Team)
	for teamId, _ := range teams {
		_, appErr := a.GetChannelById(teamId, channelId)
		if appErr != nil {
			a.logger.Error(appErr.Error())
			continue
		}
		return teamId, nil
	}
	return "", errors.New("Team id not found " + channelId)
}
func (a MattermostUserAdapter) getTeamIdAndChannelIdByChannelName(channelName string) (string, string, error) {
	result, appErr := a.client.GetAllTeams()
	if appErr != nil {
		return "", "", appErr
	}
	teams := result.Data.(map[string]*model.Team)
	for teamId, _ := range teams {
		channel, appErr := a.GetChannelByName(teamId, channelName)
		if appErr != nil || channel.Data.(*model.Channel).Name != channelName {
			continue
		}
		return teamId, channel.Data.(*model.Channel).Id, nil
	}
	return "", "", errors.New("Team id not found")
}
func (a MattermostUserAdapter) GetChannelById(teamId, channelId string) (*model.Result, *model.AppError) {
	route := fmt.Sprintf("/teams/%v/channels/%v/", teamId, channelId)
	if r, err := a.client.DoApiGet(route, "", ""); err != nil {
		return nil, err
	} else {
		defer closeBody(r)
		return &model.Result{r.Header.Get(model.HEADER_REQUEST_ID),
			r.Header.Get(model.HEADER_ETAG_SERVER), model.ChannelFromJson(r.Body)}, nil
	}
}
func (a MattermostUserAdapter) GetChannelByName(teamId, channelName string) (*model.Result, *model.AppError) {
	route := fmt.Sprintf("/teams/%v/channels/name/%v", teamId, channelName)
	if r, err := a.client.DoApiGet(route, "", ""); err != nil {
		return nil, err
	} else {
		defer closeBody(r)
		return &model.Result{r.Header.Get(model.HEADER_REQUEST_ID),
			r.Header.Get(model.HEADER_ETAG_SERVER), model.ChannelFromJson(r.Body)}, nil
	}
}
func closeBody(r *http.Response) {
	if r.Body != nil {
		ioutil.ReadAll(r.Body)
		r.Body.Close()
	}
}
func (a MattermostUserAdapter) Reply(envelop robot.Envelop, message string) error {
	return a.Send(envelop, "@" + envelop.User.Name + ": " + message)
}
func (a *MattermostUserAdapter) Run(config interface{}, gubot *robot.Gubot) error {
	conf := config.(*MattermostUserConfig)
	a.gubot = gubot
	a.logger = gubot.Logger()
	if conf.MattermostUsername == "" {
		return errors.New("mattermost_username config param is required")
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
	client := model.NewClient(mattApi.String())
	client.HttpClient = gubot.HttpClient()
	_, appErr := client.Login(conf.MattermostUsername, conf.MattermostPassword)
	if appErr != nil {
		return appErr
	}
	a.client = client
	clientWs, appErr := model.NewWebSocketClient(wsMattApi.String(), client.AuthToken)
	if appErr != nil {
		return appErr
	}
	a.clientWs = clientWs

	resp, appErr := client.GetMe("")
	if appErr != nil {
		return appErr
	}
	mattMe := resp.Data.(*model.User)
	a.me = mattMe
	clientWs.Listen()
	go func() {
		for {
			event := <-a.clientWs.EventChannel
			if event == nil {
				appErr := clientWs.Connect()
				if appErr != nil {
					a.logger.Error("Error when reconnecting to web socket: " + appErr.Error())
				}
				clientWs.Listen()
				continue
			}
			if event.Event == model.WEBSOCKET_EVENT_USER_ADDED {
				a.gubot.Emit(robot.GubotEvent{
					Action: robot.EVENT_ROBOT_CHANNEL_ENTER,
					Envelop: a.eventToEnvelop(event),
				})
				continue
			}
			if event.Event == model.WEBSOCKET_EVENT_USER_REMOVED {
				a.gubot.Emit(robot.GubotEvent{
					Action: robot.EVENT_ROBOT_CHANNEL_LEAVE,
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
				Action: robot.EVENT_ROBOT_USER_ONLINE,
				Envelop: a.userIdToDirectEnvelop(userId),
			})
		}
	}
	for userId, _ := range a.onlineUsers {
		if _, ok := onlineUsersReceived[userId]; !ok && userId != a.me.Id {
			a.gubot.Emit(robot.GubotEvent{
				Action: robot.EVENT_ROBOT_USER_OFFLINE,
				Envelop: a.userIdToDirectEnvelop(userId),
			})
		}
	}
	a.onlineUsers = onlineUsersReceived
}
func (a MattermostUserAdapter) userIdToDirectEnvelop(userId string) robot.Envelop {
	resp, appErr := a.client.GetUser(userId, "")
	if appErr != nil {
		a.logger.Error("Cannot transform event in envelop")
		return robot.Envelop{}
	}
	user := resp.Data.(*model.User)
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
	resp, appErr := a.client.GetUser(userId, "")
	if appErr != nil {
		a.logger.Error("Cannot transform event in envelop")
		return robot.Envelop{}
	}
	user := resp.Data.(*model.User)
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
func (a MattermostUserAdapter) sendingEnvelop(event *model.WebSocketEvent, mattMe  *model.User) {

	channelName := event.Data["channel_name"].(string)
	var postData PostData
	postDataRaw := event.Data["post"].(string)
	err := json.Unmarshal([]byte(postDataRaw), &postData)
	if err != nil {
		a.logger.Error("Error when unmarshalling data: " + err.Error())
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
	if mentioned {
		envelop.Message = strings.Replace(envelop.Message, "@" + a.me.Username, "", -1)
		envelop.Message = strings.Replace(envelop.Message, a.me.Username, "", -1)
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
