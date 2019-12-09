all: test

test:
	gotest --race -v ./...

test-all:
	gotest --race --count=1 -v ./...

format fmt:
	go fmt -x ./...

clean:
	go mod tidy
	go clean

.PHONY: all test test-all format fmt clean
