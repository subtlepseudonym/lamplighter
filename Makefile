BINARY=lamplighter
BUILD=$$(vtag --no-meta)

default: build

build: format
	go build -o ${BINARY} -v ./*.go

docker: format
	docker build --network=host --tag ${BINARY}:${BUILD} -f Dockerfile .

test:
	gotest --race ./...

format fmt:
	gofmt -l -w -e .

clean:
	go mod tidy
	go clean
	rm -f $(BINARY)

get-tag:
	echo ${BUILD}

.PHONY: all build format fmt clean get-tag
