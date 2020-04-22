BINARY=lamplighter
BUILD=$$(vtag)

default: all

build: format
	go build -o ${BINARY} -v ./cmd/notes

format fmt:
	go fmt -x ./...

clean:
	go mod tidy
	go clean
	rm -f $(BINARY)

get-tag:
	echo ${BUILD}

.PHONY: all build format fmt clean get-tag
