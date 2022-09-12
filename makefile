export CGO_ENABLED = 0
export GOOS = linux

BIN_NAME=dfserver
GIT_COMMIT=$(shell git rev-list -1 HEAD)
LDFLAGS = -ldflags "-s -w -X main.GitCommit=${GIT_COMMIT}"

linux-amd64: export GOARCH = amd64
linux-amd64:
	go build ${LDFLAGS} -o dist/${GOOS}_${GOARCH}/${BIN_NAME} cmd/main.go

linux-arm64: export GOARCH = arm64
linux-arm64:
	go build ${LDFLAGS} -o dist/${GOOS}_${GOARCH}/${BIN_NAME} cmd/main.go
