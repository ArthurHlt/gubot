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
	fmt.Println("Send> " + message + "\n")
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
			gubot.Receive(envelop)
		}
	}()

	return nil
}

func (a ShellAdapter) eventCmd(text string) robot.EventAction {
	text = robot.SanitizeDefault(text)
	switch text {
	case "/enter":
		return robot.EVENT_ROBOT_CHANNEL_ENTER
	case "/leave":
		return robot.EVENT_ROBOT_CHANNEL_LEAVE
	case "/online":
		return robot.EVENT_ROBOT_USER_ONLINE
	case "/offline":
		return robot.EVENT_ROBOT_USER_OFFLINE
	}
	return ""
}

func (a ShellAdapter) Name() string {
	return "shell"
}

func (a ShellAdapter) Config() interface{} {
	return struct {
	}{}
}
