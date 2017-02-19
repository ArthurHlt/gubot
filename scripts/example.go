package scripts

import (
	"github.com/ArthurHlt/gubot/robot"
	"fmt"
	"time"
	"net/http"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/olebedev/emitter"
)

func init() {
	var conf ExampleScriptConfig
	robot.GetConfig(&conf)
	e := &ExampleScript{
		annoy: make(map[string]bool),
		config: conf,
	}
	// we create the table in database only when the store has been initialized
	robot.On(robot.EVENT_ROBOT_INITIALIZED_STORE, func(event *emitter.Event) {
		robot.Store().AutoMigrate(&ExampleMenu{})
	})
	e.listen()
	robot.Router().HandleFunc("/gubot/chatsecrets/{channel}", e.handlerChatsecret)
	robot.RegisterScripts([]robot.Script{
		{
			Name: "badger",
			Matcher: "(?i)badger",
			Function: e.badger,
			Type: robot.Tsend, // will just send a message
		},
		{
			Name: "doors",
			Matcher: "(?i)open the (.*) doors", // you can have text inside (.*) inside submatch in your function
			Function: e.doors,
			Type: robot.Trespond, // will send a message by explicitly responding to a user
		},
		{
			Name: "lulz",
			Matcher: "(?i)lulz",
			Function: e.lulz,
			Type: robot.Tsend,
		},
		{
			Name: "Ultimate question",
			Description: "Answer to the ultimate question",
			Matcher: "(?i)what is the answer to the ultimate question of life",
			Function: e.ultimateQuestion,
			Type: robot.Tsend,
		},
		{
			Name: "annoy",
			Example: "`annoy me`",
			Matcher: "(?i)^annoy me",
			Function: e.annoyMe,
			Type: robot.Tsend,
		},
		{
			Name: "unannoy",
			Example: "`unannoy me`",
			Matcher: "(?i)^unannoy me",
			Function: e.unannoyMe,
			Type: robot.Tsend,
		},
		{
			Name: "menu order",
			Description: "order something to eat",
			Example: "menu order with pizza, coca & pies",
			Matcher: "(?i)menu order (with)? ([a-z]*) ([a-z]*)? ([a-z]*)?",
			Function: e.menu,
			Sanitizer: robot.SanitizeDefaultWithSpecialChar,
			Type: robot.Tsend,
		},
		{
			Name: "menu list",
			Description: "list all orders",
			Example: "menu list",
			Matcher: "(?i)menu list",
			TriggerOnMention: true,
			Function: e.menuShow,
			Type: robot.Tsend,
		},
	})
}

type ExampleMenu struct {
	gorm.Model
	User    robot.User
	UserID  int
	Plate   string
	Drink   string
	Dessert string
}
type ExampleScriptConfig struct {
	GubotAnswerToTheUltimateQuestionOfLifeTheUniverseAndEverything string
}
type ExampleScript struct {
	config ExampleScriptConfig
	annoy  map[string]bool
}
type SecretMessage struct {
	Secret string
}

func (e ExampleScript) badger(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	return []string{"Badgers? BADGERS? WE DON'T NEED NO STINKIN BADGERS"}, nil
}

func (e ExampleScript) doors(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	doorType := subMatch[0][1]
	if doorType == "pod bay" {
		return []string{"I'm afraid I can't let you do that."}, nil
	}
	return []string{"Opening " + doorType + " doors"}, nil
}

func (e ExampleScript) lulz(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	return []string{"lol", "rofl", "lmao"}, nil
}
func (e ExampleScript) menu(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	plate := subMatch[0][2]
	drink := subMatch[0][3]
	dessert := subMatch[0][4]
	var user robot.User
	robot.Store().Where(&robot.User{
		UserId: envelop.User.Id,
	}).First(&user)
	robot.Store().Create(&ExampleMenu{
		User: user,
		Plate: plate,
		Drink: drink,
		Dessert: dessert,
	})
	return e.menuShow(envelop, subMatch)
}
func (e ExampleScript) menuShow(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	var menus []ExampleMenu
	robot.Store().Find(&menus)

	res := fmt.Sprintf("There is %d order(s) in queue: \n", len(menus))
	var userDb robot.User
	for _, menu := range menus {
		robot.Store().Model(&menu).Related(&userDb)
		res += fmt.Sprintf("- %s's order: \n  - Plate: %s\n", userDb.Name, menu.Plate)
		if menu.Drink != "" {
			res += "  - Drink: " + menu.Drink + "\n"
		}
		if menu.Dessert != "" {
			res += "  - Dessert: " + menu.Dessert + "\n"
		}
	}
	return []string{res}, nil
}
func (e ExampleScript) topic(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	return []string{envelop.Message + "? that's a paddlin"}, nil
}

func (e *ExampleScript) ultimateQuestion(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	if e.config.GubotAnswerToTheUltimateQuestionOfLifeTheUniverseAndEverything == "" {
		return []string{"Missing GubotAnswerToTheUltimateQuestionOfLifeTheUniverseAndEverything config parameter: please set and try again"}, nil
	}
	return []string{e.config.GubotAnswerToTheUltimateQuestionOfLifeTheUniverseAndEverything + ", but what is the question?"}, nil
}
func (e *ExampleScript) annoyMe(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	e.annoy[envelop.ChannelName + envelop.ChannelId] = true
	go func() {
		for {
			if !e.annoy[envelop.ChannelName + envelop.ChannelId] {
				break
			}
			robot.SendMessages(envelop, "AAAAAAAAAAAEEEEEEEEEEEEEEEEEEEEEEEEIIIIIIIIHHHHHHHHHH")
			time.Sleep(2 * time.Second)
		}
	}()
	return []string{"Hey, want to hear the most annoying sound in the world?"}, nil
}
func (e *ExampleScript) unannoyMe(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
	message := "Not annoying you right now, am I?"
	if e.annoy[envelop.ChannelName + envelop.ChannelId] {
		e.annoy[envelop.ChannelName + envelop.ChannelId] = false
		message = "GUYS, GUYS, GUYS!"
	}
	return []string{message}, nil
}
func (e ExampleScript) handlerChatsecret(w http.ResponseWriter, req *http.Request) {
	var secretMessage SecretMessage
	err := json.NewDecoder(req.Body).Decode(&secretMessage)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		b, _ := json.MarshalIndent(struct {
			Code    int
			Message string
		}{http.StatusBadRequest, err.Error()}, "", "\t")
		w.Write(b)
		return
	}
	vars := mux.Vars(req)
	err = robot.SendMessages(robot.Envelop{
		ChannelName: vars["channel"],
	}, "I have a secret: " + secretMessage.Secret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		b, _ := json.MarshalIndent(struct {
			Code    int
			Message string
		}{http.StatusInternalServerError, err.Error()}, "", "\t")
		w.Write(b)
		return
	}
	w.WriteHeader(http.StatusOK)

}
func (e *ExampleScript) listen() {
	go func() {
		for event := range robot.On(robot.EVENT_ROBOT_CHANNEL_ENTER) {
			gubotEvent := robot.ToGubotEvent(event)
			err := robot.RespondMessages(gubotEvent.Envelop, "Hi", "Target Acquired", "Firing", "Hello friend.", "Gotcha", "I see you")
			if err != nil {
				robot.Logger().Error(err)
			}
		}
	}()
	go func() {
		for event := range robot.On(robot.EVENT_ROBOT_CHANNEL_LEAVE) {
			gubotEvent := robot.ToGubotEvent(event)
			err := robot.RespondMessages(gubotEvent.Envelop, "Are you still there?", "Target lost", "Searching")
			if err != nil {
				robot.Logger().Error(err)
			}
		}
	}()
	go func() {
		for event := range robot.On(robot.EVENT_ROBOT_USER_ONLINE) {
			gubotEvent := robot.ToGubotEvent(event)
			robot.SendMessages(gubotEvent.Envelop, "Hello again", "It's been a while")
		}
	}()

}