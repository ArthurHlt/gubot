package shell

import (
	"fmt"
	osuser "os/user"
	"os"
	"bufio"
	"github.com/ArthurHlt/gubot/robot"
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
	fmt.Println("\n" + message)
	return nil
}
func (a ShellAdapter) Reply(envelop robot.Envelop, message string) error {
	fmt.Print("@" + envelop.User.Name + " " + message)
	return nil
}
func (a ShellAdapter) Run(config interface{}, gubot *robot.Gubot) error {
	user, err := osuser.Current()
	if err != nil {
		return err
	}
	go func() {
		for ; ; {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print(gubot.Name() + "> ")
			text, _ := reader.ReadString('\n')
			gubot.Receive(robot.Envelop{
				User: robot.UserEnvelop{
					Name: user.Username,
					Id: user.Uid,
				},
				Message: text,
			})
		}
	}()

	return nil
}
func (a ShellAdapter) Name() string {
	return "shell"
}
func (a ShellAdapter) Config() interface{} {
	return struct {

	}{}
}