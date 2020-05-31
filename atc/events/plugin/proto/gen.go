package proto

//go:generate protoc -I ./ eventstore.proto --go_out=plugins=grpc:./
