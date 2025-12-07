# Proto

This package contains Protocol Buffer definitions for GameAP domain entities.

## Generating Code

Run the following command from this directory:

```bash
go generate ./...
```

This will generate:
- `*.pb.go` - Standard protobuf Go code
- `*_vtproto.pb.go` - Optimized marshal/unmarshal/size methods via [vtprotobuf](https://github.com/planetscale/vtprotobuf)

## Proto Files

| File | Description |
|------|-------------|
| `daemontask.proto` | Daemon task definitions |
| `entity.proto` | Common entity definitions |
| `game.proto` | Game definitions |
| `gamemod.proto` | Game modification definitions |
| `node.proto` | Node (dedicated server) definitions |
| `server.proto` | Game server definitions |
| `serversetting.proto` | Server settings definitions |
| `user.proto` | User definitions |

## Requirements

- [protoc](https://grpc.io/docs/protoc-installation/) - Protocol Buffer compiler
- [protoc-gen-go](https://pkg.go.dev/google.golang.org/protobuf/cmd/protoc-gen-go) - Go code generator
- [protoc-gen-go-vtproto](https://github.com/planetscale/vtprotobuf) - vtprotobuf generator

Install generators:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@latest
```