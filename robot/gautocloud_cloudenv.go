package robot

import (
	"github.com/cloudfoundry-community/gautocloud/cloudenv"
	"github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

const CONFIG_FILENAME = "config_gubot.yml"

type GubotConfig struct {
	Tokens       []string               `yaml:"tokens"`
	LogLevel     string                 `yaml:"log_level"`
	Name         string                 `yaml:"name"`
	Host         string                 `yaml:"host"`
	SkipInsecure bool                   `yaml:"skip_insecure"`
	Services     []ServiceLocal         `yaml:"services"`
	Config       map[string]interface{} `yaml:"config" cloud:"-"`
}
type ConfFileCloudEnv struct {
	id          string
	credentials map[string]interface{}
	services    []ServiceLocal
}
type ServiceLocal struct {
	Name        string                 `yaml:"name"`
	Tags        []string               `yaml:"tags"`
	Credentials map[string]interface{} `yaml:"credentials"`
}

func NewConfFileCloudEnv() cloudenv.CloudEnv {
	return &ConfFileCloudEnv{}

}

func (c ConfFileCloudEnv) Name() string {
	return "gubot"
}
func (c ConfFileCloudEnv) getServicesWithTag(tag string) []cloudenv.Service {
	services := make([]cloudenv.Service, 0)
	for _, serviceLocal := range c.services {
		if c.serviceLocalHasTag(serviceLocal, tag) {
			services = append(services, cloudenv.Service{
				Credentials: serviceLocal.Credentials,
			})
		}
	}
	return services
}
func (c ConfFileCloudEnv) GetServicesFromTags(tags []string) []cloudenv.Service {
	if c.isGubotSvcAsked(tags...) {
		return []cloudenv.Service{{
			Credentials: c.credentials,
		}}
	}
	services := make([]cloudenv.Service, 0)
	for _, tag := range tags {
		services = append(services, c.getServicesWithTag(tag)...)
	}
	return services
}
func (c ConfFileCloudEnv) isGubotSvcAsked(tagsOrNames ...string) bool {
	for _, tagOrName := range tagsOrNames {
		if match(".*"+c.Name()+".*", tagOrName) {
			return true
		}
	}
	return false
}
func (c ConfFileCloudEnv) serviceLocalHasTag(serviceLocal ServiceLocal, tag string) bool {
	for _, tagLocal := range serviceLocal.Tags {
		if match(tag, tagLocal) {
			return true
		}
	}
	return false
}
func (c *ConfFileCloudEnv) Load() error {
	if !c.IsInCloudEnv() {
		return nil
	}
	confPath, err := c.getConfPath()
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(confPath)
	if err != nil {
		return err
	}
	var conf GubotConfig
	err = yaml.Unmarshal(b, &conf)
	if err != nil {
		return err
	}
	conf.Config["skip_insecure"] = conf.SkipInsecure
	conf.Config["name"] = conf.Name
	conf.Config["host"] = conf.Host
	conf.Config["tokens"] = conf.Tokens
	conf.Config["log_level"] = conf.LogLevel
	c.credentials = conf.Config

	c.services = conf.Services
	if c.services == nil {
		c.services = make([]ServiceLocal, 0)
	}
	return nil
}
func (c ConfFileCloudEnv) GetServicesFromName(name string) []cloudenv.Service {
	if c.isGubotSvcAsked(name) {
		return []cloudenv.Service{{
			Credentials: c.credentials,
		}}
	}
	services := make([]cloudenv.Service, 0)
	for _, serviceLocal := range c.services {
		if match(name, serviceLocal.Name) {
			services = append(services, cloudenv.Service{
				Credentials: serviceLocal.Credentials,
			})
		}
	}
	return services
}
func (c ConfFileCloudEnv) getConfPath() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(pwd, CONFIG_FILENAME), nil
}
func (c ConfFileCloudEnv) IsInCloudEnv() bool {
	confPath, err := c.getConfPath()
	if err != nil {
		return false
	}
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return false
	}
	return true
}
func (c ConfFileCloudEnv) GetAppInfo() cloudenv.AppInfo {
	id := c.id
	if id == "" {
		id = uuid.NewV4().String()
		c.id = id
	}
	return cloudenv.AppInfo{
		Id:         c.id,
		Name:       Name(),
		Properties: make(map[string]interface{}),
	}
}
