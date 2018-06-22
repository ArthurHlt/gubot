#!/usr/bin/env bash

go get github.com/ArthurHlt/gubot

if hash curl 2>/dev/null; then
    curl https://raw.githubusercontent.com/ArthurHlt/gubot/master/main.go > main.go
else
    wget -o "main.go" "https://raw.githubusercontent.com/ArthurHlt/gubot/master/main.go"
fi
if hash curl 2>/dev/null; then
    curl https://raw.githubusercontent.com/ArthurHlt/gubot/master/config_gubot.tmpl.yml > config_gubot.yml
else
    wget -o "config_gubot.yml" "https://raw.githubusercontent.com/ArthurHlt/gubot/master/config_gubot.tmpl.yml"
fi
mkdir static