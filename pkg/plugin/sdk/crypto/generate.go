package crypto

//go:generate protoc -I../../../.. --go-plugin_out=../../../.. --go-plugin_opt=paths=source_relative pkg/plugin/sdk/crypto/crypto.proto
