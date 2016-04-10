# rtcdc-p2p
p2p data connection on webrtc

## depends

```sh
go get github.com/keroserene/go-webrtc
```

## signaling server for appengine-go
```sh
go get -d  github.com/nobonobo/rtcdc-p2p/signaling
cd $GOPATH/src/github.com/nobonobo/rtcdc-p2p/signaling/server
make update APPID=whatever
```

https://signaling-2016.appspot-com/

## server build and run

```sh
go get -u github.com/nobonobo/rtcdc-p2p/server
server -room sample-room
```

## client build and run

```sh
go get -u github.com/nobonobo/rtcdc-p2p/client
client -room sample-room -id client1
```
