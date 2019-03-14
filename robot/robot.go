package robot

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/ArthurHlt/gubot/robot/assets"
	"github.com/cloudfoundry-community/gautocloud"
	"github.com/cloudfoundry-community/gautocloud/cloudenv"
	_ "github.com/cloudfoundry-community/gautocloud/connectors/databases/gorm"
	"github.com/cloudfoundry-community/gautocloud/loader"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/olebedev/emitter"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

const (
	SQLITE_DB           = "gubot.db"
	icon_route          = "/static_compiled/gubot_icon.png"
	REMOTE_SCRIPTS_NAME = "remote_scripts"
)

const (
	EVENT_ROBOT_STARTED           EventAction = "started"
	EVENT_ROBOT_CHANNEL_ENTER     EventAction = "channel_enter"
	EVENT_ROBOT_CHANNEL_LEAVE     EventAction = "channel_leave"
	EVENT_ROBOT_USER_ONLINE       EventAction = "user_online"
	EVENT_ROBOT_USER_OFFLINE      EventAction = "user_offline"
	EVENT_ROBOT_INITIALIZED       EventAction = "initialized"
	EVENT_ROBOT_INITIALIZED_STORE EventAction = "initialized_store"
	EVENT_ROBOT_RECEIVED          EventAction = "received"
	EVENT_ROBOT_SEND              EventAction = "send"
	EVENT_ROBOT_RESPOND           EventAction = "respond"
	EVENT_ROBOT_NO_SCRIPT_FOUND   EventAction = "no_script_found"
)

type EventAction string
type GubotEvent struct {
	Name    EventAction
	Envelop Envelop
	Message string
}

type Gubot struct {
	GubotEmitter       *emitter.Emitter
	name               string
	host               string
	adapters           []Adapter
	router             *mux.Router
	tokens             []string
	skipInsecure       bool
	httpClient         *http.Client
	gautocloud         loader.Loader
	store              *gorm.DB
	scripts            *Scripts
	slashCommands      *[]SlashCommand
	mutexScript        *sync.Mutex
	mutexSlashCommand  *sync.Mutex
	scriptMiddlewares  []ScriptMiddleware
	commandMiddlewares []CommandMiddleware
}

func NewGubot() *Gubot {
	cloudenvs := gautocloud.CloudEnvs()

	finalCloudEnvs := make([]cloudenv.CloudEnv, 0)
	for _, env := range cloudenvs {
		if _, ok := env.(*cloudenv.LocalCloudEnv); ok {
			continue
		}
		finalCloudEnvs = append(finalCloudEnvs, env)
	}
	finalCloudEnvs = append(finalCloudEnvs, NewConfFileCloudEnv())
	ldCloud := loader.NewLoader(finalCloudEnvs)
	for _, connector := range gautocloud.Connectors() {
		ldCloud.RegisterConnector(connector)
	}
	scripts := Scripts(make([]Script, 0))
	slashCommands := make([]SlashCommand, 0)
	gubot := &Gubot{
		GubotEmitter:       emitter.New(uint(100)),
		name:               "gubot",
		adapters:           make([]Adapter, 0),
		router:             mux.NewRouter(),
		tokens:             make([]string, 0),
		gautocloud:         ldCloud,
		httpClient:         &http.Client{},
		scripts:            &scripts,
		mutexScript:        new(sync.Mutex),
		mutexSlashCommand:  new(sync.Mutex),
		scriptMiddlewares:  make([]ScriptMiddleware, 0),
		commandMiddlewares: make([]CommandMiddleware, 0),
		slashCommands:      &slashCommands,
	}
	gubot.gautocloud.RegisterConnector(NewGubotGenericConnector(GubotConfig{}))
	return gubot
}

func (g *Gubot) createHttpClient() {
	tr := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: g.skipInsecure},
	}
	g.httpClient.Transport = tr
}

func (g *Gubot) registerUser(envelop Envelop) {
	if envelop.User.Id == "" {
		return
	}
	dbUser := &User{
		UserId:  envelop.User.Id,
		Name:    envelop.User.Name,
		Channel: envelop.User.ChannelName,
	}
	var count int
	g.store.Model(dbUser).Where(&User{
		UserId: envelop.User.Id,
		Name:   envelop.User.Name,
	}).Count(&count)
	if count == 0 {
		g.store.Create(dbUser)
	}
}

func (g Gubot) Emit(event GubotEvent) {
	<-g.GubotEmitter.Emit(string(event.Name), event)
}

func (g Gubot) On(eventAction EventAction, middlewares ...func(*emitter.Event)) <-chan emitter.Event {
	return g.GubotEmitter.On(string(eventAction), middlewares...)
}

func (g Gubot) Once(eventAction EventAction, middlewares ...func(*emitter.Event)) <-chan emitter.Event {
	return g.GubotEmitter.Once(string(eventAction), middlewares...)
}

func (g Gubot) Emitter() *emitter.Emitter {
	return g.GubotEmitter
}

func (g *Gubot) SetLogLevel(level string) {
	lvl, err := log.ParseLevel(level)
	if err != nil {
		panic(err)
	}
	log.SetLevel(lvl)
}

func (g Gubot) Name() string {
	return g.name
}

func (g *Gubot) SetName(name string) {
	g.name = name
}

func (g *Gubot) SkipInsecure() {
	g.skipInsecure = true
	g.createHttpClient()
}

func (g Gubot) HttpClient() *http.Client {
	return g.httpClient
}

func (g *Gubot) RegisterAdapter(adp Adapter) {
	g.adapters = append(g.adapters, adp)
	g.gautocloud.RegisterConnector(NewGubotGenericConnector(adp.Config()))
}

func (g *Gubot) runAdapters() {
	for _, adp := range g.adapters {
		val := reflect.New(reflect.TypeOf(adp.Config()))
		config := val.Interface()
		g.gautocloud.Inject(config)
		err := adp.Run(config, g)
		if err != nil {
			log.Fatalf("Error when loading adapter '%s' : %s", adp.Name(), err.Error())
		}
	}
}

func (g Gubot) GetConfig(config interface{}) error {
	if reflect.TypeOf(config).Kind() != reflect.Ptr {
		return errors.New("you must pass a pointer")
	}
	v := reflect.ValueOf(config)
	if v.IsNil() {
		v = reflect.New(reflect.TypeOf(v))
	}
	g.gautocloud.RegisterConnector(NewGubotGenericConnector(v.Elem().Interface()))
	return g.gautocloud.Inject(config)
}

func (g *Gubot) Use(middlewares ...Middleware) {
	for _, mid := range middlewares {
		g.scriptMiddlewares = append(g.scriptMiddlewares, mid.ScriptMiddleware)
		g.commandMiddlewares = append(g.commandMiddlewares, mid.CommandMiddleware)
	}
}

func (g *Gubot) UseScript(middlewares ...ScriptMiddleware) {
	g.scriptMiddlewares = append(g.scriptMiddlewares, middlewares...)
}

func (g *Gubot) UseCommand(middlewares ...CommandMiddleware) {
	g.commandMiddlewares = append(g.commandMiddlewares, middlewares...)
}

func (g *Gubot) RegisterSlashCommand(slashCommand SlashCommand) error {
	defer g.mutexSlashCommand.Unlock()
	g.mutexSlashCommand.Lock()
	err := g.checkSlashCommand(slashCommand)
	if err != nil {
		return err
	}
	if g.findSlashCommandIndex(slashCommand) != -1 {
		return errors.New("slashCommand '%s' already registered")
	}
	*g.slashCommands = append(*g.slashCommands, slashCommand)
	log.Debugf("%s registered.", slashCommand)
	return nil
}

func (g *Gubot) RegisterSlashCommands(slashCommands []SlashCommand) error {
	for _, cmd := range slashCommands {
		err := g.RegisterSlashCommand(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) UnregisterSlashCommand(slashCommand SlashCommand) error {
	defer g.mutexSlashCommand.Unlock()
	g.mutexSlashCommand.Lock()
	err := g.checkSlashCommand(slashCommand)
	if err != nil {
		return err
	}
	i := g.findSlashCommandIndex(slashCommand)
	if i == -1 {
		return nil
	}
	cmds := *g.slashCommands
	cmds = append(cmds[:i], cmds[i+1:]...)
	*g.slashCommands = cmds
	log.Debugf("%s unregistered.", slashCommand)
	return nil
}

func (g *Gubot) UnregisterSlashCommands(slashCommands []SlashCommand) error {
	for _, cmd := range slashCommands {
		err := g.UnregisterSlashCommand(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) UpdateSlashCommand(slashCommand SlashCommand) error {
	defer g.mutexScript.Unlock()
	g.mutexScript.Lock()
	err := g.checkSlashCommand(slashCommand)
	if err != nil {
		return err
	}
	i := g.findSlashCommandIndex(slashCommand)
	if i == -1 {
		return nil
	}
	cmds := *g.slashCommands
	cmds[i] = slashCommand
	*g.slashCommands = cmds
	log.Debugf("%s updated.", slashCommand)
	return nil
}

func (g *Gubot) UpdateSlashCommands(slashCommands []SlashCommand) error {
	for _, cmd := range slashCommands {
		err := g.UpdateSlashCommand(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) RegisterScript(script Script) error {
	defer g.mutexScript.Unlock()
	g.mutexScript.Lock()
	err := g.checkScript(script)
	if err != nil {
		return err
	}
	if g.findScriptIndex(script) != -1 {
		return errors.New("Script '%s' already registered")
	}
	if script.Sanitizer == nil {
		script.Sanitizer = SanitizeDefault
	}
	*g.scripts = append(*g.scripts, script)
	log.Debugf("%s registered.", script.String())
	return nil
}

func (g *Gubot) DispatchCommand(ident SlashCommandToken, envelop Envelop) (interface{}, error) {
	var adapter SlashCommandAdapter
	for _, adp := range g.adapters {
		if adp.Name() != ident.AdapterName {
			continue
		}
		if _, ok := adp.(SlashCommandAdapter); ok {
			adapter = adp.(SlashCommandAdapter)
		}
	}
	if adapter == nil {
		return nil, fmt.Errorf("No adapter found for command %s", ident.CommandName)
	}
	var finalCmd SlashCommand
	for _, cmd := range *g.slashCommands {
		if cmd.Trigger == ident.CommandName {
			finalCmd = cmd
			break
		}
	}
	if finalCmd.Function == nil {
		return nil, fmt.Errorf("No function found for command %s", ident.CommandName)
	}
	result, err := g.generateCommandFunction(finalCmd, finalCmd.Function)(envelop)
	if err != nil {
		return nil, err
	}
	if result == "" {
		return nil, nil
	}
	return adapter.Format(result)

}

func (g *Gubot) RegisterScripts(scripts []Script) error {
	for _, script := range scripts {
		err := g.RegisterScript(script)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) UnregisterScript(script Script) error {
	defer g.mutexScript.Unlock()
	g.mutexScript.Lock()
	err := g.checkScript(script)
	if err != nil {
		return err
	}
	i := g.findScriptIndex(script)
	if i == -1 {
		return nil
	}
	scripts := *g.scripts
	scripts = append(scripts[:i], scripts[i+1:]...)
	*g.scripts = scripts
	log.Debugf("%s unregistered.", script.String())
	return nil
}

func (g *Gubot) UnregisterScripts(scripts []Script) error {
	for _, script := range scripts {
		err := g.UnregisterScript(script)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Gubot) UpdateScript(script Script) error {
	defer g.mutexScript.Unlock()
	g.mutexScript.Lock()
	err := g.checkScript(script)
	if err != nil {
		return err
	}
	i := g.findScriptIndex(script)
	if i == -1 {
		return nil
	}
	scripts := *g.scripts
	scripts[i] = script
	*g.scripts = scripts
	log.Debugf("%s updated.", script.String())
	return nil
}

func (g *Gubot) UpdateScripts(scripts []Script) error {
	for _, script := range scripts {
		err := g.UpdateScript(script)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g Gubot) checkScript(script Script) error {
	if script.Function == nil || script.Matcher == "" || script.Type == "" ||
		script.Name == "" {
		return errors.New("Script " + script.Name + " can't have function, matcher, type or name empty.")
	}
	return nil
}

func (g *Gubot) findScriptIndex(findScript Script) int {
	for index, script := range *g.scripts {
		if script.Name == findScript.Name &&
			script.Matcher == findScript.Matcher &&
			script.Type == findScript.Type {
			return index
		}
	}
	return -1
}

func (g Gubot) checkSlashCommand(slashCommand SlashCommand) error {
	if slashCommand.Function == nil || slashCommand.Title == "" || slashCommand.Trigger == "" {
		return errors.New("Slash command " + slashCommand.Title + " can't have function, title or trigger_word empty.")
	}
	return nil
}

func (g *Gubot) findSlashCommandIndex(slashCommand SlashCommand) int {
	for index, cmd := range *g.slashCommands {
		if cmd.Title == slashCommand.Title &&
			cmd.Trigger == slashCommand.Trigger {
			return index
		}
	}
	return -1
}

func (g Gubot) Store() *gorm.DB {
	return g.store
}

func (g *Gubot) Receive(envelop Envelop) {
	envelop.FromReceived = true
	log.Debugf("Received envelop=%v", envelop)
	g.Emit(GubotEvent{
		Name:    EVENT_ROBOT_RECEIVED,
		Envelop: envelop,
	})
	g.registerUser(envelop)
	toSends := g.getMessages(envelop, Tsend)
	toReplies := g.getMessages(envelop, Trespond)
	toDirect := g.getMessages(envelop, Tdirect)

	if len(toSends) == 0 && len(toReplies) == 0 && len(toSends) == 0 {
		g.Emit(GubotEvent{
			Name:    EVENT_ROBOT_NO_SCRIPT_FOUND,
			Envelop: envelop,
		})
	}

	err := g.SendMessages(envelop, toSends...)
	if err != nil {
		log.Error("Error when sending messages: " + err.Error())
	}
	err = g.RespondMessages(envelop, toReplies...)
	if err != nil {
		log.Error("Error when replying messages: " + err.Error())
	}
	err = g.SendDirectMessages(envelop, toDirect...)
	if err != nil {
		log.Error("Error when sending direct messages: " + err.Error())
	}
}

func (g *Gubot) SendMessages(envelop Envelop, toSends ...string) error {
	for _, adp := range g.adapters {
		log.Debugf("Adapter '%s' chose a random message from list [\"%s\"] and sent it.",
			adp.Name(),
			strings.Join(toSends, "\", \""),
		)
		err := g.sendingEnvelop(envelop, adp.Send, EVENT_ROBOT_SEND, toSends)
		if err != nil {
			log.Debugf("Error when sending on adapter '%s' : %s", adp.Name(), err.Error())
		}

	}
	return nil
}

func (g *Gubot) SendDirectMessages(envelop Envelop, toReplies ...string) error {
	for _, adp := range g.adapters {
		log.Debugf("Adapter '%s' chose a random message from list [\"%s\"] and reply to user.",
			adp.Name(),
			strings.Join(toReplies, "\", \""),
		)
		var reply func(Envelop, string) error
		reply = adp.Reply
		if _, ok := adp.(SendDirectAdapter); ok {
			reply = adp.(SendDirectAdapter).SendDirect
		}
		err := g.sendingEnvelop(envelop, reply, EVENT_ROBOT_RESPOND, toReplies)
		if err != nil {
			log.Error("Error when responding on adapter '" + adp.Name() + "' : " + err.Error())
		}
	}
	return nil
}

func (g *Gubot) RespondMessages(envelop Envelop, toReplies ...string) error {
	if len(toReplies) > 0 && envelop.User.Name == "" {
		return errors.New("You must provide a user name in envelop")
	}
	for _, adp := range g.adapters {
		log.Debugf("Adapter '%s' chose a random message from list [\"%s\"] and reply to user.",
			adp.Name(),
			strings.Join(toReplies, "\", \""),
		)
		err := g.sendingEnvelop(envelop, adp.Reply, EVENT_ROBOT_RESPOND, toReplies)
		if err != nil {
			log.Error("Error when responding on adapter '" + adp.Name() + "' : " + err.Error())
		}
	}
	return nil
}

func (g Gubot) choseRandomMessage(messages []string) string {
	if len(messages) == 0 {
		return ""
	}
	if len(messages) == 1 {
		return messages[0]
	}
	return messages[rand.Intn(len(messages))]
}

func (g *Gubot) sendingEnvelop(envelop Envelop, adpFn func(Envelop, string) error, eventAction EventAction, messages []string) error {
	if len(messages) == 0 {
		return nil
	}
	message := g.choseRandomMessage(messages)
	if envelop.IconUrl == "" && g.host != "" {
		host := g.host
		if strings.HasPrefix(host, "https") {
			host = strings.TrimPrefix(host, "https")
			host = "http" + host
		}
		envelop.IconUrl = host + icon_route
	}
	g.Emit(GubotEvent{
		Name:    eventAction,
		Envelop: envelop,
		Message: message,
	})
	return adpFn(envelop, message)
}

func (g *Gubot) LoadStore() error {
	var store *gorm.DB
	err := g.gautocloud.Inject(&store)
	if err != nil {
		dbFile := filepath.Join(os.TempDir(), SQLITE_DB)
		store, err = gorm.Open("sqlite3", dbFile)
		if err != nil {
			panic("failed to connect database")
		}
		log.Info("Sqlite file created in path: " + dbFile)
	}
	store.AutoMigrate(&User{})
	store.AutoMigrate(&RemoteScript{})
	store.AutoMigrate(&SlashCommandToken{})
	var rmtScripts []RemoteScript
	store.Find(&rmtScripts)
	for _, rmtScript := range rmtScripts {
		g.RegisterScript(g.remoteScriptToScript(rmtScript))
	}
	g.store = store
	return nil
}

func (g Gubot) Host() string {
	return g.host
}

func (g *Gubot) loadHost() {
	if g.host != "" {
		return
	}
	cloudenvName := g.gautocloud.CurrentCloudEnv().Name()
	if cloudenvName == (cloudenv.HerokuCloudEnv{}).Name() {
		g.host = "https://" + g.gautocloud.GetAppInfo().Properties["host"].(string)
	}
	if cloudenvName == (cloudenv.CfCloudEnv{}).Name() {
		g.host = "https://" + g.gautocloud.GetAppInfo().Properties["uris"].([]string)[0]
	}
	resp, err := g.httpClient.Get(g.host)
	if err != nil || resp.StatusCode != http.StatusOK {
		g.host = strings.TrimPrefix(g.host, "https")
		g.host = "http" + g.host
	}
}

func (g Gubot) InitDefaultRoute() {
	g.router.Handle("/", g.ApiAuthMatcher()(http.HandlerFunc(g.incoming))).Methods("POST")
	g.router.Handle("/", http.HandlerFunc(g.showScripts)).Methods("GET")

	g.router.Handle("/slash-command", http.HandlerFunc(g.slashCommand)).Methods("POST", "GET")

	mux.NewRouter()
	apiRouter := g.router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/websocket", g.serveWebSocket)
	apiRouter.Handle("/send", g.ApiAuthMatcher()(http.HandlerFunc(g.sendMessagesRemoteScripts))).Methods("POST")
	apiRouter.Handle("/respond", g.ApiAuthMatcher()(http.HandlerFunc(g.respondMessagesRemoteScripts))).Methods("POST")

	apiRmtRouter := apiRouter.PathPrefix("/remote").Subrouter()
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.registerRemoteScripts))).Methods("POST")
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.deleteRemoteScripts))).Methods("DELETE")
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.updateRemoteScripts))).Methods("PUT")
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.listRemoteScripts))).Methods("GET")

	staticDir := "static"
	if stat, err := os.Stat(staticDir); err == nil && stat.IsDir() {
		g.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	}
	g.router.PathPrefix("/static_compiled/").Handler(http.StripPrefix("/static_compiled/", http.FileServer(assets.StaticAssetFs())))
}

func (g *Gubot) ApiAuthMatcher() func(http.Handler) http.Handler {
	fn := func(h http.Handler) http.Handler {
		return TokenAuthHandler{h, g}
	}
	return fn
}

func (g Gubot) IsSecured(w http.ResponseWriter, req *http.Request) bool {
	auth := false
	req.ParseForm()
	if g.IsValidToken(req.Header.Get("X-Auth-Token")) {
		auth = true
	}
	if g.IsValidToken(req.Header.Get("Authorization")) {
		auth = true
	}
	if g.IsValidToken(req.Form.Get("token")) {
		auth = true
	}
	if g.IsValidToken(req.PostForm.Get("token")) {
		auth = true
	}
	if g.IsValidToken(req.URL.Query().Get("token")) {
		auth = true
	}

	return auth
}

func (g Gubot) getMessages(envelop Envelop, typeScript TypeScript) []string {
	toSends := make([]string, 0)
	g.mutexScript.Lock()
	scripts := *g.scripts
	g.mutexScript.Unlock()
	for _, script := range scripts.ListFromType(typeScript) {
		if script.TriggerOnMention && envelop.NotMentioned {
			continue
		}
		message := script.Sanitizer(envelop.Message)
		if !match(script.Matcher, message) {
			continue
		}
		log.Debug("%s respond on envelop=%v", script.String(), envelop)
		function := g.generateScriptFunction(script, script.Function)
		messages, err := function(envelop, allSubMatch(script.Matcher, message))
		if err != nil {
			log.Error(fmt.Sprintf("Error on script '%s': %s", script.Name, err.Error()))
			continue
		}
		toSends = append(toSends, messages...)
	}
	return toSends
}

func (g Gubot) generateScriptFunction(script Script, handler EnvelopHandler) EnvelopHandler {
	for i := len(g.scriptMiddlewares) - 1; i >= 0; i-- {
		middleware := g.scriptMiddlewares[i]
		handler = middleware(script, handler)
	}
	return handler
}

func (g Gubot) generateCommandFunction(command SlashCommand, handler CommandHandler) CommandHandler {
	for i := len(g.commandMiddlewares) - 1; i >= 0; i-- {
		middleware := g.commandMiddlewares[i]
		handler = middleware(command, handler)
	}
	return handler
}

func match(matcher, content string) bool {
	regex := regexp.MustCompile(matcher)
	return regex.MatchString(content)
}

func allSubMatch(matcher, content string) [][]string {
	regex := regexp.MustCompile(matcher)
	return regex.FindAllStringSubmatch(content, -1)
}

func (g Gubot) Router() *mux.Router {
	return g.router
}

func (g *Gubot) SetTokens(tokens []string) {
	g.tokens = tokens
}

func (g Gubot) Tokens() []string {
	return g.tokens
}

func (g Gubot) IsValidToken(tokenToCheck string) bool {
	for _, token := range g.tokens {
		if tokenToCheck == token {
			return true
		}
	}
	return false
}

func (g Gubot) GetScripts() []Script {
	return []Script(*g.scripts)
}

func (g *Gubot) InitializeHelp() {
	g.RegisterScript(Script{
		Name:        "help",
		Description: "Provide the list of available scripts",
		Matcher:     "(?i)^help$",
		Function: func(envelop Envelop, subMatch [][]string) ([]string, error) {
			list := "Available scripts: \n"
			g.mutexScript.Lock()
			scripts := *g.scripts
			g.mutexScript.Unlock()
			for _, script := range scripts {
				if script.Name == REMOTE_SCRIPTS_NAME {
					continue
				}
				list += fmt.Sprintf(
					"- %s",
					strings.Title(script.Name),
				)
				if script.TriggerOnMention {
					list += "*"
				}
				if script.Example == "" {
					list += " -- regex: `" + script.Matcher + "`"
				} else {
					list += " -- e.g.: `" + script.Example + "`"
				}
				if script.Description != "" {
					list += " -- " + strings.Title(script.Description)
				}
				list += "\n"
			}
			list += "`*`: Script will be only triggered when talking explicitly to the bot."
			return []string{list}, nil
		},
		TriggerOnMention: true,
		Type:             Tsend,
	})
}

func (g Gubot) SlashCommandUrl() string {
	return g.Host() + "/slash-command"
}

func (g Gubot) IconUrl() string {
	return g.Host() + icon_route
}

func (g Gubot) initSlashCommand() error {
	var result error
	for _, adp := range g.adapters {
		if _, ok := adp.(SlashCommandAdapter); !ok {
			continue
		}
		for _, cmd := range *g.slashCommands {
			slashTokens, err := adp.(SlashCommandAdapter).Register(cmd)
			if err != nil {
				result = multierror.Append(result, err)
			}
			for _, slashToken := range slashTokens {
				slashToken.AdapterName = adp.Name()
				err = g.store.Create(slashToken).Error
				if err != nil {
					result = multierror.Append(result, err)
					continue
				}
			}
		}
	}
	return result
}

func (g *Gubot) Start(addr string) error {
	defer g.GubotEmitter.Off("*")
	var conf GubotConfig
	err := g.gautocloud.Inject(&conf)
	if err != nil {
		return err
	}
	if conf.LogLevel != "" {
		g.SetLogLevel(conf.LogLevel)
	}

	if conf.SkipInsecure {
		g.SkipInsecure()
	}
	g.createHttpClient()
	if len(conf.Tokens) > 0 {
		g.SetTokens(conf.Tokens)
	}
	if len(g.tokens) == 0 {
		defaultToken := uuid.NewV4().String()
		g.tokens = []string{defaultToken}
		log.Info("Generated token: " + defaultToken)
	}
	if conf.Host != "" {
		g.host = conf.Host
	} else {
		g.loadHost()
	}
	g.LoadStore()
	g.Emit(GubotEvent{
		Name: EVENT_ROBOT_INITIALIZED_STORE,
	})
	log.Info("Listening on `" + addr + "`")
	g.runAdapters()
	g.InitDefaultRoute()
	g.InitializeHelp()
	err = g.initSlashCommand()
	if err != nil {
		log.Error(err)
	}
	err = g.registerProgramScripts(conf.ProgramScripts)
	if err != nil {
		return err
	}
	g.Emit(GubotEvent{
		Name: EVENT_ROBOT_INITIALIZED,
	})
	g.Emit(GubotEvent{
		Name: EVENT_ROBOT_STARTED,
	})
	return http.ListenAndServe(addr, g.router)
}
