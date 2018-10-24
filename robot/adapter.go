package robot

type Adapter interface {
	Name() string
	Send(Envelop, string) error
	Reply(Envelop, string) error
	Run(interface{}, *Gubot) error
	Config() interface{}
}

type SendDirectAdapter interface {
	SendDirect(Envelop, string) error
}

type SlashCommandAdapter interface {
	Register(slashCommand SlashCommand) ([]SlashCommandToken, error)
	Format(message string) (interface{}, error)
}
