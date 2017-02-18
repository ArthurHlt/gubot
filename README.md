![Gubot](/static/gubot.png)
# Gubot

Gubot is a chat bot like hubot written in go. He's pretty cool as hubot is. He's extendable with scripts and can work on many different chat services.

Gubot is not just a reimplementation of hubot but a rewriting with new cool stuffs like:
- Just might run on any cloud without any changes (it uses [gautocloud](https://github.com/cloudfoundry-community/gautocloud))
- Add a mechanism of unified configuration for scripts and adapters
- Can use any rdbms without any configuration (thanks to [gautocloud](https://github.com/cloudfoundry-community/gautocloud), and yes rdbms, it's faster and more reliable on small data than nosql databases)
- Untied to a specific language to add scripts and receive events from Gubot (see [API](#api) to know how to add remote scripts)
- Has a mechanism of sanitizer to be able to transform a message before giving it to a script (The ultimate goal would be use natural language when user chat with the bot)
