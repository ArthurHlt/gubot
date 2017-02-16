#!/usr/bin/env bash
hash glide 2>/dev/null
if [ "$?" == "1" ]; then
    curl https://glide.sh/get | sh
fi

glide create
glide get github.com/ArthurHlt/gubot/robot
glide get github.com/ArthurHlt/gubot/adapter
glide get github.com/ArthurHlt/gubot/scripts

if hash curl 2>/dev/null; then
    curl https://raw.githubusercontent.com/ArthurHlt/gubot/master/main.go > main.go
else
    wget -o "main.go" "https://raw.githubusercontent.com/ArthurHlt/gubot/master/main.go"
fi