package robot

type Envelop struct {
	Message      string                 `json:"message"`
	ChannelName  string                 `json:"channel_name"`
	ChannelId    string                 `json:"channel_id"`
	IconUrl      string                 `json:"icon_url"`
	NotMentioned bool                   `json:"not_mentioned"`
	User         UserEnvelop            `json:"user"`
	Properties   map[string]interface{} `json:"properties"`
}
type UserEnvelop struct {
	Name        string                 `json:"name"`
	Id          string                 `json:"id"`
	ChannelName string                 `json:"channel_name"`
	ChannelId   string                 `json:"channel_id"`
	Properties  map[string]interface{} `json:"properties"`
}
