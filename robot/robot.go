package robot

import (
	"regexp"
	"log"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"crypto/tls"
	"net"
	"time"
	"reflect"
	"os"
	"github.com/cloudfoundry-community/gautocloud/loader"
	"github.com/cloudfoundry-community/gautocloud"
	_ "github.com/cloudfoundry-community/gautocloud/connectors/databases/gorm"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"strconv"
	"errors"
	"path/filepath"
	"github.com/satori/go.uuid"
	"github.com/cloudfoundry-community/gautocloud/cloudenv"
	"math/rand"
	"strings"
	"github.com/ArthurHlt/gubot/robot/assets"
	"github.com/olebedev/emitter"
)

const (
	SQLITE_DB = "gubot.db"
	icon_route = "/static_compiled/gubot_icon.png"
	REMOTE_SCRIPTS_NAME = "remote_scripts"
)
const (
	EVENT_ROBOT_STARTED EventAction = "started"
	EVENT_ROBOT_CHANNEL_ENTER EventAction = "enter"
	EVENT_ROBOT_CHANNEL_LEAVE EventAction = "leave"
	EVENT_ROBOT_USER_ONLINE EventAction = "online"
	EVENT_ROBOT_USER_OFFLINE EventAction = "offline"
	EVENT_ROBOT_INITIALIZED EventAction = "initialized"
	EVENT_ROBOT_INITIALIZED_STORE EventAction = "initialized_store"
	EVENT_ROBOT_RECEIVED EventAction = "received"
	EVENT_ROBOT_SEND EventAction = "send"
	EVENT_ROBOT_RESPOND EventAction = "respond"
)

type EventAction string
type GubotEvent struct {
	Action        EventAction
	Envelop       Envelop
	ChosenMessage string
}

type Gubot struct {
	GubotEmitter *emitter.Emitter
	name         string
	host         string
	adapters     []Adapter
	router       *mux.Router
	tokens       []string
	skipInsecure bool
	httpClient   *http.Client
	gautocloud   *loader.Loader
	store        *gorm.DB
	scripts      Scripts
}

func NewGubot() *Gubot {
	cloudenvs := gautocloud.CloudEnvs()
	cloudenvs = append(cloudenvs, NewConfFileCloudEnv())
	ldCloud := loader.NewLoader(cloudenvs)
	for _, connector := range gautocloud.Connectors() {
		ldCloud.RegisterConnector(connector)
	}
	gubot := &Gubot{
		GubotEmitter: emitter.New(uint(100)),
		name: "gubot",
		adapters: make([]Adapter, 0),
		router: mux.NewRouter(),
		tokens: make([]string, 0),
		gautocloud: ldCloud,
		httpClient: &http.Client{},
		scripts: make([]Script, 0),
	}
	gubot.gautocloud.RegisterConnector(NewGubotGenericConnector(GubotConfig{}))
	return gubot
}
func (g *Gubot) createHttpClient() {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: g.skipInsecure},
	}
	g.httpClient.Transport = tr
}
func (g *Gubot) registerUser(envelop Envelop) {
	if envelop.User.Id == "" {
		return
	}
	dbUser := &User{
		UserId: envelop.User.Id,
		Name: envelop.User.Name,
		Channel: envelop.User.ChannelName,
	}
	var count int
	g.store.Model(dbUser).Where(&User{
		UserId: envelop.User.Id,
		Name: envelop.User.Name,
	}).Count(&count)
	if count == 0 {
		g.store.Create(dbUser)
	}
}
func (g Gubot) Emit(event GubotEvent) {
	<-g.GubotEmitter.Emit(string(event.Action), event)
}
func (g Gubot) On(eventAction EventAction, middlewares ...func(*emitter.Event)) <- chan emitter.Event {
	return g.GubotEmitter.On(string(eventAction), middlewares...)
}
func (g Gubot) Once(eventAction EventAction, middlewares ...func(*emitter.Event)) <- chan emitter.Event {
	return g.GubotEmitter.Once(string(eventAction), middlewares...)
}
func (g Gubot) Emitter() *emitter.Emitter {
	return g.GubotEmitter
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
			log.Fatal(errors.New("Error when loading adapter '" + adp.Name() + "' : " + err.Error()))
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
func (g *Gubot) RegisterScript(script Script) error {
	if script.Function == nil || script.Matcher == "" || script.Type == "" ||
		script.Name == "" {
		return errors.New("Script " + script.Name + " can't have function, matcher, type or name empty.")
	}
	g.scripts = append(g.scripts, script)
	return nil
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
func (g Gubot) Store() *gorm.DB {
	return g.store
}
func (g *Gubot) Receive(envelop Envelop) {
	g.Emit(GubotEvent{
		Action: EVENT_ROBOT_RECEIVED,
		Envelop: envelop,
	})
	g.registerUser(envelop)
	toSends := g.getMessages(envelop, Tsend)
	toReplies := g.getMessages(envelop, Trespond)

	err := g.SendMessages(envelop, toSends...)
	if err != nil {
		log.Println("Error when sending messages: " + err.Error())
	}
	err = g.RespondMessages(envelop, toReplies...)
	if err != nil {
		log.Println("Error when replying messages: " + err.Error())
	}
}
func (g *Gubot) SendMessages(envelop Envelop, toSends ...string) error {
	for _, adp := range g.adapters {
		err := g.sendingEnvelop(envelop, adp.Send, EVENT_ROBOT_SEND, toSends)
		if err != nil {
			log.Println("Error when sending on adapter '" + adp.Name() + "' : " + err.Error())
		}
	}
	return nil
}
func (g *Gubot) RespondMessages(envelop Envelop, toReplies ...string) error {
	if len(toReplies) > 0 && envelop.User.Name == "" {
		return errors.New("You must provide a user name in envelop")
	}
	for _, adp := range g.adapters {
		err := g.sendingEnvelop(envelop, adp.Reply, EVENT_ROBOT_RESPOND, toReplies)
		if err != nil {
			log.Println("Error when responding on adapter '" + adp.Name() + "' : " + err.Error())
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
			host += "http" + host
		}
		envelop.IconUrl = host + icon_route
	}
	g.Emit(GubotEvent{
		Action: eventAction,
		Envelop: envelop,
		ChosenMessage: message,
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
		log.Println("Sqlite file created in path: " + dbFile)
	}
	store.AutoMigrate(&User{})
	store.AutoMigrate(&RemoteScript{})
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
	mux.NewRouter()
	apiRmtRouter := g.router.PathPrefix("/api/remote").Subrouter()
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.registerRemoteScripts))).Methods("POST")
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.deleteRemoteScripts))).Methods("DELETE")
	apiRmtRouter.Handle("/scripts", g.ApiAuthMatcher()(http.HandlerFunc(g.updateRemoteScripts))).Methods("PUT")
	apiRmtRouter.Handle("/send", g.ApiAuthMatcher()(http.HandlerFunc(g.sendMessagesRemoteScripts))).Methods("POST")
	apiRmtRouter.Handle("/respond", g.ApiAuthMatcher()(http.HandlerFunc(g.respondMessagesRemoteScripts))).Methods("POST")

	staticDir := "static"
	if stat, err := os.Stat(staticDir); err == nil && stat.IsDir() {
		g.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	}
	g.router.PathPrefix("/static_compiled/").Handler(http.StripPrefix("/static_compiled/", http.FileServer(assets.StaticAssetFs())))
}
func (g Gubot) InitDefaultScripts() {
	g.RegisterScripts([]Script{
		{
			Name: REMOTE_SCRIPTS_NAME,
			Type: Tsend,
			Matcher: ".*",
			Function: func(envelop Envelop, subMatch [][]string) ([]string, error) {
				return g.sendToScript(envelop, Tsend)
			},
		},
		{
			Name: REMOTE_SCRIPTS_NAME,
			Type: Trespond,
			Matcher: ".*",
			Function: func(envelop Envelop, subMatch [][]string) ([]string, error) {
				return g.sendToScript(envelop, Trespond)
			},
		},
	})
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
	for _, script := range g.scripts.ListFromType(typeScript) {
		if !match(script.Matcher, envelop.Message) {
			continue
		}
		messages, err := script.Function(envelop, allSubMatch(script.Matcher, envelop.Message))
		if err != nil {
			log.Println(fmt.Sprintf("Error on script '%s': %s", script.Name, err.Error()))
			continue
		}
		toSends = append(toSends, messages...)
	}
	return toSends
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
	return []Script(g.scripts)
}
func (g *Gubot) InitializeHelp() {
	g.RegisterScript(Script{
		Name: "help",
		Description: "Provide the list of available scripts",
		Matcher: "(?i)^help$",
		Function: func(envelop Envelop, subMatch [][]string) ([]string, error) {
			list := "Available scripts: \n"
			for _, script := range g.scripts {
				if script.Name == REMOTE_SCRIPTS_NAME {
					continue
				}
				list += fmt.Sprintf(
					"- %s",
					strings.Title(script.Name),
				)
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
			return []string{list}, nil
		},
		Type: Tsend,
	})
}
func (g *Gubot) Start(port int) error {
	defer g.GubotEmitter.Off("*")
	var conf GubotConfig
	err := g.gautocloud.Inject(&conf)
	if err != nil {
		return err
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
		log.Println("Generated token: " + defaultToken)
	}
	if conf.Host != "" {
		g.host = conf.Host
	} else {
		g.loadHost()
	}
	g.LoadStore()
	g.Emit(GubotEvent{
		Action: EVENT_ROBOT_INITIALIZED_STORE,
	})
	log.Println("Listening on port: " + strconv.Itoa(port))
	g.runAdapters()
	g.InitDefaultScripts()
	g.InitDefaultRoute()
	g.InitializeHelp()
	g.Emit(GubotEvent{
		Action: EVENT_ROBOT_INITIALIZED,
	})
	g.Emit(GubotEvent{
		Action: EVENT_ROBOT_STARTED,
	})
	return http.ListenAndServe(":" + strconv.Itoa(port), g.router)
}