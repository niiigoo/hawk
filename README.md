# HAWK

This is a CLI-tool to build microservices with go.

Hawk takes the service definition from a `.proto` file and generates the boilerplate for a server supporting gRPC and
HTTP (with JSON).

This project can be considered as the successor of [metaverse/truss](https://github.com/metaverse/truss), the code is
partly taken over.

## Installation

Installing Hawk is straight forward

```shell
go install github.com/niiigoo/hawk@latest
```

### Dependencies

In order to compile the `.proto` file, `protoc` is required
([official installation instruction](https://grpc.io/docs/protoc-installation/)).

In addition, the go specific extensions are necessary:

```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Update your `PATH` environment variable to contain the location of `protoc` and the go-related binaries
(by default `$(go env GOPATH)/bin`).

## Usage

### Initialize a project

Initializing a project means, generating the `go.mod` file and the `.proto` file containing the basic structure of the
service.

```shell
# Command:
hawk init <repository> <name>
# Example:
hawk init github.com/orga/sample-service sample
```

### Generate boilerplate

#### Service structure

```
Project dir
├── cmd # do not touch
│   ├── <project>
│   │   ├── main.go
├── handlers
│   ├── handlers.go # entrypoint for business logic
│   ├── hooks.go # stop gracefully
│   ├── middleware.go # apply middleware
├── svc # do not touch
├── go.mod
├── go.sum
├── *.proto # define your service
├── *.pb.go # do not touch
```

#### Command

```shell
hawk generate
# Or use a shortcut
hawk gen
hawk g
```

## Planned features

- Add a standardized logger
- Provide basic middlewares
- Implement compression (HTTP)
- Generate documentation
   - Markdown
   - Swagger
- Advanced tools
   - `hawk generate entity <name>`
- WebSocket support
