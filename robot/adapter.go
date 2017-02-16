package robot

type Adapter interface {
	Name() string
	Send(Envelop, string) error
	Reply(Envelop, string) error
	Run(interface{}, *Gubot) error
	Config() interface{}
}
