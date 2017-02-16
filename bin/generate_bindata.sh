#!/usr/bin/env bash
hash go-bindata 2>/dev/null
if [ "$?" == "1" ]; then
    go get -u github.com/jteeuwen/go-bindata/...
fi

hash go-bindata-assetfs 2>/dev/null
if [ "$?" == "1" ]; then
    go get -u github.com/elazarl/go-bindata-assetfs/...
fi

BASEDIR=$(dirname $0)

go-bindata-assetfs -pkg assets "$BASEDIR/../static/..."
mv  "bindata_assetfs.go" "$BASEDIR/../robot/assets/bindata_assetfs.go"