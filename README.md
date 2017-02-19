![Gubot](/static/gubot.png)
# Gubot

Gubot is a chat bot like hubot written in go. He's pretty cool as hubot is. He's extendable with scripts and can work on many different chat services (called adapters in Gubot).

Gubot is not just a reimplementation of hubot but a rewriting with new cool stuffs like:
- Just might run on any cloud without any changes (it uses [gautocloud](https://github.com/cloudfoundry-community/gautocloud))
- Add a mechanism of unified configuration for scripts and adapters
- Can use any rdbms without any configuration (thanks to [gautocloud](https://github.com/cloudfoundry-community/gautocloud), and yes rdbms, it's faster and more reliable on small data than nosql databases)
- Untied to a specific language to add scripts and receive events from Gubot (see [Remote scripts](#remote-scripts) to know how to add remote scripts)
- Has a mechanism of sanitizer to be able to transform a message before giving it to a script (The ultimate goal would be use natural language when user chat with the bot)

It supports 3 different chat services by default:
- [Slack](/adapter/slack/adapter.go)
- [Shell](/adapter/shell/adapter.go) *(mainly for testing your scripts)*
- [Mattermost websocket and API](/adapter/mattermost_user/adapter.go)

## Summary

- [Getting started](#getting-started)
  - [Automatically](#automatically)
  - [Manually](#manually)
  - [Run in a cloud](#run-in-a-cloud)
    - [Example on Cloud Foundry](#cloud-foundry)
    - [Example on Heroku](#heroku)
- [Unified configuration system](#unified-configuration-system)
- [Create your own script(s)](#create-your-own-scripts)
  - [Overview](#overview) 
  - [Listen for Gubot events](#listen-for-gubot-events) 
  - [Sanitizers mechanism](#sanitizers-mechanism) 
  - [Use the available router](#use-the-available-router) 
  - [Use the store system](#use-the-store-system) 
- [Create your own adapter](#create-your-own-adapter)
- [Remote scripts](#remote-scripts)
- [API](#api)
  - [CRUD Remote scripts](#crud-remote-scripts)
    - [Create remote scripts](#create-remote-scripts)
    - [Update remote scripts](#udpate-remote-scripts)
    - [Delete remote scripts](#delete-remote-scripts)
    - [List remote scripts](#list-remote-scripts)
  - [Give send and respond messages to Gubot](#give-send-and-respond-messages-to-gubot)
    - [Send message](#send-message)
    - [Respond message](#respond-message)
  - [Use websocket to listens events](#use-websocket-to-listens-events)
    - [Authentication](#authentication)
    - [Events](#events)

## Getting started

Requirements:
- [Go](https://golang.org/doc/install)

### Automatically

1. Create a folder where you will have source for your bot and go inside, e.g.: `mkdir mygubot && cd mygubot`
2. Run `curl https://raw.githubusercontent.com/ArthurHlt/gubot/master/bin/bootstrap.sh | sh`, it will install bootstrap 
a Gubot with an [example](/scripts/example.go) loaded
3. *(option if target is a cloud environment)* Change the `config_gubot.yml` as you want
4. you can now run it directly with `go run main.go` or see how to [run in a cloud](#run-in-a-cloud)

### Manually

1. Create a folder inside `$GOPATH/src` and go inside
2. run `go get github.com/ArthurHlt/gubot`
3. Create a `main.go` file which load Gubot:
```go
package main

import (
	"github.com/ArthurHlt/gubot/robot"

	// adapters
	_ "github.com/ArthurHlt/gubot/adapter/shell"

	// scripts
	_ "github.com/ArthurHlt/gubot/scripts"

	"log"
	"os"
)

func main() {
	addr := ":8080"
	port := os.Getenv("PORT")
	if port != "" {
		addr = ":" + port
	}
	log.Fatal(robot.Start(addr))
}
```
3. *(option if target is a cloud environment)* Create a `config_gubot.yml` file from the [template](/config_gubot.tmpl.yml).
4. you can now run it directly with `go run main.go` or see how to [run in a cloud](#run-in-a-cloud)

### Run in a cloud

Gubot uses [gautocloud](https://github.com/cloudfoundry-community/gautocloud) which is a library to load services automatically.

By default Gautoucloud support [Cloud Foundry](http://cloudfoundry.org/) and [Heroku](https://www.heroku.com/) but you can use others 
[gautocloud cloud environment](https://github.com/cloudfoundry-community/gautocloud#cloud-environments) made by others to use a different cloud (see how to add a cloud env 
in gautocloud doc, it's simple).

We will give here 2 examples for those 2 cloud by translating the `config_gubot.tmpl.yml`.

#### Cloud Foundry

You will need to set a user provided service which contains the configuration of gubot (this permit to change config without push and push all the time).

1. create a file called for example `services.json`, this file contains config you want to have for scripts and adapters, 
here config from `config_gubot.tmpl.yml` will become:
```json
{
  "name": "gubot",
  "skip_insecure": false,
  "log_level": "",
  "gubot_answer_to_the_ultimate_question_of_life_the_universe_and_everything": "42",
  "slack_income_url": "http://localhost/hooks/975rc3rxyjbs5pz8e4rjn7mm5y",
  "tokens": [
    "atokentosecuredata"
  ]
}
```
2. run `cf cups gubot-config -p services.json`
3. create a manifest file like this one:
```yml
name: gubot
memory: 64M
buildpack: go_buildpack
services:
- gubot-config
```
4. run `cf push`
5. Your gubot is ready
6. *(Option but recommended)* By default storage system use sqlite, you can use different storage system by binding 
another storage system as mysql to the app and restage it

#### Heroku

To set the configuration you will need to set environment variables which start with `GUBOT_` and match configuration parameters from scripts and adapters.

The `config_gubot.tmpl.yml` file will be those env vars:
```
GUBUT_NAME="gubo"
GUBOT_SKIP_INSECURE="false"
GUBOT_LOG_LEVEL=""
GUBOT_GUBOT_ANSWER_TO_THE_ULTIMATE_QUESTION_OF_LIFE_THE_UNIVERSE_AND_EVERYTHING="42"
GUBOT_SLACK_INCOME_URL="http://localhost/hooks/975rc3rxyjbs5pz8e4rjn7mm5y"
GUBOT_TOKENS="atokentosecuredata,asecondtoken"
```

*(Option but recommended)* By default storage system use sqlite, you can use different storage system by binding 
another storage system as mysql by, for example, use [cleardb](https://devcenter.heroku.com/articles/cleardb) on your app and that's all.

## Unified configuration system

This system is here to provide a simple way to get configuration parameters in a script or/and adapter.

This allow 3 things:
1. Scripts and adapters will always use this to retrieve configuration (In hubot, scripts and adpaters was creating own configuration system for each, can be either based on files or env var. It was painful for final users)
2. This untied configuration to an app but to the service above like Cloud Foundry and Heroku
3. Not need to developers to write a configuration system

To create config in a script simply create a structure (see the [decoder doc from gautocloud](https://godoc.org/github.com/cloudfoundry-community/gautocloud/decoder)
 to see what you can do on this struct) and ask to gubot to give it the final config, example:
```go
type MySuperConfig struct {
        MyToken         string `cloud:"myToken"`
        MySpecialConfig string // by default config parameter name will be my_special_config
}
func init(){
        var conf MySuperConfig
        robot.GetConfig(&conf) // ask to Gubot to give corresponding configuration in the var conf
        fmt.Println(conf) // you will that you will have config retrieved from gubot
}
```
with this `config_gubot.yml`: 
```yml
config:
  myToken: mysupertoken
  my_special_config: "a special value"
```

In your scripts docs simply give config parameter name because it can change in different cloud environment.


## Create your own script(s)

### Overview

The [example.go](/scripts/example.go) file give all possibilities you have to create your own script(s).

The main things to understand is that you must register your script(s) in Gubot by providing an init function. 
This permit to other users who want use your script to simply add `import _ "path/to/your/scripts"` to load them.

Here a little example:
```
func init(){
    robot.RegisterScripts([]robot.Script{
    		{
    			Name: "badger",
    			Matcher: "(?i)badger",
    			Function: func(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
                          	return []string{"Badgers? BADGERS? WE DON'T NEED NO STINKIN BADGERS"}, nil
                          },
    			Type: robot.Tsend, // robot.Tsend to send without responding to user and robot.Trespond to respond to a user
    		},
    })
}
```

### Listen for Gubot events

You can listen events from Gubot such as:
- `initialized_store`: When Gubot finished to initialize store
- `initialized`: When Gubot finished to initialize
- `started`: When Gubot as really started
- `channel_enter`*: When a user enter in a channel
- `channel_leave`*: When a user leave a channel
- `user_online`: When a user is connected to the chat 
- `user_offline`: When a user disconnected from the chat 
- `received`: When Gubot received an envelop
- `send`: When Gubot received an order to send message to adapter(s)
- `respond`: When Gubot received an order to send message by responding to user to adapter(s)

`*`: This must be adapter which sent this event, only mattermost_user in default adapter implements this.

Gubot use the library [emitter](https://github.com/olebedev/emitter) to implements listening, you can use directly 
this library by calling `robot.Emitter()` but you will feel more comfortable by using wrappers:

```go
for event := range robot.On(robot.EVENT_ROBOT_STARTED) { // you're listening on event "started" 
    gubotEvent := robot.ToGubotEvent(event) // this convert to gubot event directly
    err := robot.RespondMessages(gubotEvent.Envelop, "Gubot has started")
    if err != nil {
        robot.Logger().Error(err)
    }
}
for event := range robot.On("*") { // you're listening on all incoming events
        // do what you want
}
// here an example to create table in the store after store has been initialized
robot.On(robot.EVENT_ROBOT_INITIALIZED_STORE, func(event *emitter.Event) { // just run once the function after the function
		robot.Store().AutoMigrate(&struct{
		        MyFakeFieldForTable string
		}{})
})
```

### Sanitizers mechanism

When registering a script you can pass a sanitize function (a `func(string) string` function), this function will be call 
on the message received from the chat service to sanitize the message. If none are set Gubot will add the function `SanitizeDefault()`
 (which can be see in file [/robot/sanitizer.go](/robot/sanitizer.go)) which remove multiples spaces, newlines and tabs.
 
You can found sanitize function to use directly in [/robot/sanitizer.go](/robot/sanitizer.go).

Here an example:

```go
func init(){
    robot.RegisterScripts([]robot.Script{
    		{
    			Name: "badger",
    			Matcher: "(?i)badger",
    			Function: func(envelop robot.Envelop, subMatch [][]string) ([]string, error) {
                          	return []string{"Badgers? BADGERS? WE DON'T NEED NO STINKIN BADGERS"}, nil
                          },
    			Type: robot.Tsend,
    			Sanitizer: func(text string) string { // this function is directly available with robot.SanitizeDefaultWithSpecialChar
                            r := regexp.MustCompile("[^(\\w|\\s)]") //it removes all things which are not spaces and words (comma, colons, plus ...)
                            return SanitizeDefault(r.ReplaceAllString(text, ""))
                           },
    		},
    })
}
```

### Use the available router

Gubot, as hubot, let you access to a router to register your own routes.

It uses the [gorilla/mux](https://github.com/gorilla/mux) library, you can have access to the router by calling `robot.Router()`.

### Use the store system

You can use Gubot to store data over a rdbms. Gubot use [GORM](https://github.com/jinzhu/gorm) over [gautocloud](https://github.com/cloudfoundry-community/gautocloud), 
this permit you to use any rdbms available in the [GORM connector](https://github.com/cloudfoundry-community/gautocloud/blob/master/docs/connectors.md).

You can access to the store by calling `robot.Store()` and see the [docs of gorm](http://jinzhu.me/gorm/) to know how to use it.

Gubot already store 2 tables (see [/robot/db_model.go](/robot/db_model.go)):
- `User`: Any user from a chat are registered inside, you can use this table to make a reference between your own table and user
- `RemoteScript`: Store all remote scripts registers throught the [API](#api).

## Create your own adapter

To create an adapter you must implements the [adapter interface](/robot/adapter.go) and add an `init` function to register your adapter in Gubot.
This permit to other users who want use your adapter to simply add `import _ "path/to/your/adapter"` to load it.

Example of `init` function: 

```go
func init() {
	robot.RegisterAdapter(NewShellAdapter())
}

```

You can find good examples in the folder [/adapter](/adapter), the simplest is the `shell` adapter and the most complete is `mattermost_user`.

## Remote scripts

This part will explain how to use a different language to script and register/use it in gubot.

This example will show you how to do with php.

First, we will create a php script which will receive by `POST` a json in the form of:
```json
{
	"message": "",
	"channel_name": "",
	"channel_id": "",
	"icon_url": "",
	"not_mentioned": false,
	"user": {
		"name": "",
		"id": "",
		"channel_name": "",
		"channel_id": "",
		"properties": null
	},
	"properties": null,
	"sub_match": null
}
```
and will return an array of string (one of message in the list will be chosen randomly by Gubot)

Here the php script that we will call `small.php`:

```php
<?php
$json = file_get_contents('php://input'); // get post as a stream to retrieve json
$obj = json_decode($json);
file_put_contents('php://stdout', print_r($obj, true)); // just show in stdout the content of the envelop
echo json_encode(["hello from php"]); // give just one message in the list `hello from php`
```

Now we can serve this little file over a small php server (example: run `php -S localhost:8081` in the folder of the file)

Run your Gubot, here we will assume that he is listening on `8080`.

We will have now to register your script in Gubot for that we will use `curl` as an example

```bash
curl -XPOST -H 'Authorization: atokenregisteredingubot' -H "Content-type: application/json" -d '{
    "name": "send-php",
    "matcher": "hello send .*php.*",
    "url": "http://localhost:8081/test.php",
    "type": "send"
  }' 'http://localhost:8080/api/remote/scripts'
```

Now on your chat service, type `hello send my php` and you will receive `hello from php`.

For more informations about api let's have look in the part just after.

## API

The api was made to let the possibility to use scripts and listen events from Gubot remotely.

It give the ability to use different language than golang to add scripts but also required to have a url endpoint to call the script.

### CRUD Remote scripts

**Important**: You must include an `Authorization` header with one tokens stored in Gubot.

#### Create remote scripts

**Endpoint**: `/api/remote/scripts`
**Method**: `POST`

**Expected body** *(this can be an array)*:
```json
{
	"name": "", //required, name of the script
	"matcher": "", //required
	"type": "", //required: respond or send
	"url": "", //required, url of your remote script to send envelop
	"description": "",
	"example": "",
	"trigger_on_mention": false
}
```

**Example in curl**:
```bash
curl -XPOST -H 'Authorization: atokenregisteredingubot' -H "Content-type: application/json" -d '{
    "name": "send-php",
    "matcher": "hello send .*php.*",
    "url": "http://localhost:8081/test.php",
    "type": "send"
  }' 'http://localhost:8080/api/remote/scripts'
```

#### Update remote scripts

**Endpoint**: `/api/remote/scripts`
**Method**: `PUT`

**Expected body** *(this can be an array)*:
```json
{
	"name": "", //required, name of the script which was registered
	"matcher": "",
	"type": "",
	"url": "",
	"description": "",
	"example": "",
	"trigger_on_mention": false
}
```

**Example in curl**:
```bash
curl -XPUT -H 'Authorization: atokenregisteredingubot' -H "Content-type: application/json" -d '{
    "name": "send-php",
    "matcher": "hello send toto",
    "url": "http://localhost:8081/test.php",
    "type": "send"
  }' 'http://localhost:8080/api/remote/scripts'
```

#### Delete remote scripts

**Endpoint**: `/api/remote/scripts`
**Method**: `DELETE`

**Expected body** *(this can be an array)*:
```json
{
	"name": "" //required, name of the script which was registered
}
```

**Example in curl**:
```bash
curl -XPUT -H 'Authorization: atokenregisteredingubot' -H "Content-type: application/json" -d '{
    "name": "send-php"
  }' 'http://localhost:8080/api/remote/scripts'
```

#### List remote scripts

**Endpoint**: `/api/remote/scripts`
**Method**: `GET`

**Return the body**:
```json
[
  {
	"name": "", //required, name of the script which was registered
	"matcher": "", //required
	"type": "", //required: respond or send
	"url": "", //required, url of your remote script to send envelop
	"description": "",
	"example": "",
	"trigger_on_mention": false
  },
  {
  	"name": "", //required, name of the script which was registered
  	"matcher": "", //required
  	"type": "", //required: respond or send
  	"url": "", //required, url of your remote script to send envelop
  	"description": "",
  	"example": "",
  	"trigger_on_mention": false
    }
    //...
]
```

### Give send and respond messages to Gubot

**Important**: You must include an `Authorization` header with one tokens stored in Gubot.

#### Send message

**Endpoint**: `/api/send`
**Method**: `POST`

**Expected body** *(this can be an array)*:
```json
{
	"envelop": {
	    "channel_name": "", //required
		"message": "",
		"channel_id": "",
		"icon_url": "",
		"not_mentioned": false,
		"user": {
			"name": "",
			"id": "",
			"channel_name": "",
			"channel_id": "",
			"properties": {}
		},
		"properties": {}
	},
	"messages": ["content of message"] //required
}
```

**Example in curl**:
```bash
curl -XPOST -H 'Authorization: atokenregisteredingubot' -H "Content-type: application/json" -d '{
  "envelop": {
    "channel_name": "town-square"
  },
  "messages": [
    "chica mend"
  ]
}' 'http://localhost:8080/api/send'
```

#### Respond message

**Endpoint**: `/api/respond`
**Method**: `POST`

**Expected body** *(this can be an array)*:
```json
{
	"envelop": {
	    "channel_name": "", // required
		"message": "",
		"channel_id": "",
		"icon_url": "",
		"not_mentioned": false,
		"user": {
			"name": "", // required
			"id": "",
			"channel_name": "",
			"channel_id": "",
			"properties": {}
		},
		"properties": {}
	},
	"messages": ["content of message"] //required
}
```
```bash
curl -XPOST -H 'Authorization: atokenregisteredingubot' -H "Content-type: application/json" -d '{
  "envelop": {
    "channel_name": "town-square"
    "user": {
        "name": "ahalet"
    }
  },
  "messages": [
    "chica mend"
  ]
}' 'http://localhost:8080/api/respond'
```

### Use websocket to listens events

You can use websocket to listens events from Gubot, to do so you can connect to this endpoint `/api/websocket`.

This implementation is highly inspired from [mattermost](https://api.mattermost.com/#tag/WebSocket)

#### Authentication

To authenticate with an authentication challenge, first connect the WebSocket and then send the following JSON over the connection:

```json
{
  "seq": 1,
  "token": "atokenregisteredingubot"
}
```

If successful, you will receive a standard OK response from the webhook:

```json
{
  "seq": 1,
  "status": "OK"
}
```

You can now listen for events

#### Events

Events on the WebSocket will have the form:
```json
{
  "seq": 2,
  "status": "OK",
  "event": {
    "Name": "channel_enter",
    "Envelop": {
        "message": "message received from adapters",
        "channel_name": "",
        "channel_id": "",
        "icon_url": "",
        "not_mentioned": false,
        "user": {
            "name": "",
            "id": "",
            "channel_name": "",
            "channel_id": "",
            "properties": {}
        },
        "properties": {}
    },
    "Message": "message send by script"
  }
}
```

The even name is related to [Listen for Gubot events](#listen-for-gubot-events).

You will have 5seconds to send back an acknowledgment instead the server will retry 2 times to send you the events and 
after shutdown the connection.

Here the expected a message to send back to server:
```json
{
  "status": "OK",
  "seq_reply": 2
}
```

You can take a look to the go implementation of the websocket client availble on 
[/helper/websocket_client.go](/helper/websocket_client.go) to write your own.