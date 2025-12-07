package proto

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-vtproto_out=. --go-vtproto_opt=paths=source_relative,features=marshal+unmarshal+size entity.proto game.proto gamemod.proto daemontask.proto node.proto server.proto serversetting.proto user.proto
