package robot

import (
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/olebedev/emitter"
	"net/http"
)

var robot *Gubot = NewGubot()

func Robot() *Gubot {
	return robot
}
func Emitter() *emitter.Emitter {
	return robot.Emitter()
}
func On(eventAction EventAction, middlewares ...func(*emitter.Event)) <-chan emitter.Event {
	return robot.On(eventAction, middlewares...)
}
func Once(eventAction EventAction, middlewares ...func(*emitter.Event)) <-chan emitter.Event {
	return robot.Once(eventAction, middlewares...)
}
func Emit(event GubotEvent) {
	robot.Emit(event)
}
func Name() string {
	return robot.Name()
}
func SetName(name string) {
	robot.SetName(name)
}
func SkipInsecure() {
	robot.SkipInsecure()
}
func HttpClient() *http.Client {
	return robot.HttpClient()
}
func RegisterAdapter(adp Adapter) {
	robot.RegisterAdapter(adp)
}
func Use(middlewares ...Middleware) {
	robot.Use(middlewares...)
}
func UseScript(middlewares ...ScriptMiddleware) {
	robot.UseScript(middlewares...)
}
func UseCommand(middlewares ...CommandMiddleware) {
	robot.UseCommand(middlewares...)
}
func GetConfig(config interface{}) error {
	return robot.GetConfig(config)
}
func Store() *gorm.DB {
	return robot.Store()
}
func Receive(envelop Envelop) {
	robot.Receive(envelop)
}
func SendMessages(envelop Envelop, toSends ...string) error {
	return robot.SendMessages(envelop, toSends...)
}
func SendDirectMessages(envelop Envelop, toSends ...string) error {
	return robot.SendDirectMessages(envelop, toSends...)
}
func RespondMessages(envelop Envelop, toReplies ...string) error {
	return robot.RespondMessages(envelop, toReplies...)
}
func LoadStore() error {
	return robot.LoadStore()
}
func InitDefaultRoute() {
	robot.InitDefaultRoute()
}
func ApiAuthMatcher() func(http.Handler) http.Handler {
	return robot.ApiAuthMatcher()
}

func SlashCommandUrl() string {
	return robot.SlashCommandUrl()
}

func IconUrl() string {
	return robot.IconUrl()
}

func DispatchCommand(ident SlashCommandToken, envelop Envelop) (interface{}, error) {
	return robot.DispatchCommand(ident, envelop)
}

func RegisterSlashCommand(slashCommand SlashCommand) error {
	return robot.RegisterSlashCommand(slashCommand)
}
func RegisterSlashCommands(slashCommand []SlashCommand) error {
	return robot.RegisterSlashCommands(slashCommand)
}
func UnregisterSlashCommand(script SlashCommand) error {
	return robot.UnregisterSlashCommand(script)
}
func UnregisterSlashCommands(slashCommand []SlashCommand) error {
	return robot.UnregisterSlashCommands(slashCommand)
}
func UpdateSlashCommand(script SlashCommand) error {
	return robot.UpdateSlashCommand(script)
}
func UpdateSlashCommands(slashCommand []SlashCommand) error {
	return robot.UpdateSlashCommands(slashCommand)
}

func RegisterScript(script Script) error {
	return robot.RegisterScript(script)
}
func RegisterScripts(scripts []Script) error {
	return robot.RegisterScripts(scripts)
}
func UnregisterScript(script Script) error {
	return robot.UnregisterScript(script)
}
func UnregisterScripts(scripts []Script) error {
	return robot.UnregisterScripts(scripts)
}
func UpdateScript(script Script) error {
	return robot.UpdateScript(script)
}
func UpdateScripts(scripts []Script) error {
	return robot.UpdateScripts(scripts)
}
func Router() *mux.Router {
	return robot.Router()
}
func SetTokens(tokens []string) {
	robot.SetTokens(tokens)
}
func Tokens() []string {
	return robot.Tokens()
}
func Host() string {
	return robot.Host()
}
func GetScripts() []Script {
	return robot.GetScripts()
}
func IsValidToken(tokenToCheck string) bool {
	return robot.IsValidToken(tokenToCheck)
}
func SetLogLevel(level string) {
	robot.SetLogLevel(level)
}
func Start(addr string) error {
	return robot.Start(addr)
}
func ToGubotEvent(event emitter.Event) GubotEvent {
	return event.Args[0].(GubotEvent)
}
