package robot

import (
	"github.com/cloudfoundry-community/gautocloud/connectors"
	"github.com/satori/go.uuid"
)

type GubotGenericConnector struct {
	schema interface{}
	id     string
}

func NewGubotGenericConnector(schema interface{}) connectors.Connector {
	return &GubotGenericConnector{
		schema: schema,
		id: uuid.NewV4().String() + ":gubot",
	}
}
func (c GubotGenericConnector) Id() string {
	return c.id
}
func (c GubotGenericConnector) Name() string {
	return ".*config.*"
}
func (c GubotGenericConnector) Tags() []string {
	return []string{"gubot", "config.*"}
}
func (c GubotGenericConnector) Load(schema interface{}) (interface{}, error) {
	return schema, nil
}
func (c GubotGenericConnector) Schema() interface{} {
	return c.schema
}

