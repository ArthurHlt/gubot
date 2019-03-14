package shell

import (
	"bufio"
	"fmt"
	"github.com/ArthurHlt/gubot/robot"
	"os"
	osuser "os/user"
	"strings"
)

func init() {
	robot.RegisterAdapter(NewShellAdapter())
}

type ShellAdapter struct {
}

func NewShellAdapter() robot.Adapter {
	return &ShellAdapter{}
}

func (a ShellAdapter) Send(envelop robot.Envelop, message string) error {
	fmt.Print("Send> " + message + "\n")
	return nil
}

func (a ShellAdapter) Reply(envelop robot.Envelop, message string) error {
	fmt.Print("Reply> " + message + "\n")
	return nil
}

func (a ShellAdapter) SendDirect(envelop robot.Envelop, message string) error {
	fmt.Print("Direct> " + message + "\n")
	return nil
}

func (a ShellAdapter) Run(config interface{}, gubot *robot.Gubot) error {
	user, err := osuser.Current()
	if err != nil {
		return err
	}
	go func() {
		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print(strings.Title(gubot.Name()) + "> ")
			text, _ := reader.ReadString('\n')
			envelop := robot.Envelop{
				User: robot.UserEnvelop{
					Name: user.Username,
					Id:   user.Uid,
				},
				Message: text,
			}
			if !strings.HasPrefix(envelop.Message, "/") {
				gubot.Receive(envelop)
				continue
			}
			splitCmd := strings.Split(envelop.Message, " ")
			cmd := strings.TrimPrefix(splitCmd[0], "/")
			if len(splitCmd) > 1 {
				envelop.Message = strings.Join(splitCmd[1:], " ")
			} else {
				envelop.Message = ""
			}
			var slashToken robot.SlashCommandToken
			var c int
			err := robot.Store().Where("id = ?", cmd).Find(&slashToken).Count(&c).Error
			if err != nil || c == 0 {
				continue
			}
			message, _ := gubot.DispatchCommand(slashToken, envelop)
			if message == nil || message.(string) == "" {
				continue
			}
			fmt.Print("Send> " + message.(string) + "\n")
		}
	}()

	return nil
}

// func (a ShellAdapter) eventCmd(text string) robot.EventAction {
// 	text = robot.SanitizeDefault(text)
// 	switch text {
// 	case "/enter":
// 		return robot.EVENT_ROBOT_CHANNEL_ENTER
// 	case "/leave":
// 		return robot.EVENT_ROBOT_CHANNEL_LEAVE
// 	case "/online":
// 		return robot.EVENT_ROBOT_USER_ONLINE
// 	case "/offline":
// 		return robot.EVENT_ROBOT_USER_OFFLINE
// 	}
// 	return ""
// }

func (a ShellAdapter) Name() string {
	return "shell"
}

func (a ShellAdapter) Config() interface{} {
	return struct {
	}{}
}

func (a ShellAdapter) Format(message string) (interface{}, error) {
	return message, nil
}

func (a ShellAdapter) Register(slashCommand robot.SlashCommand) ([]robot.SlashCommandToken, error) {
	var slashToken robot.SlashCommandToken
	var c int
	robot.Store().Where("id = ?", slashCommand.Trigger).Find(&slashToken).Count(&c)
	if c == 1 {
		return []robot.SlashCommandToken{}, nil
	}
	return []robot.SlashCommandToken{{
		ID:          slashCommand.Trigger,
		AdapterName: a.Name(),
		CommandName: slashCommand.Trigger,
	}}, nil
}
