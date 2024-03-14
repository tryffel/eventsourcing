fmt:
	find . -type f -name go.mod \
		-execdir go fmt ./... \;

test:
	find . -type f -name go.mod | \
		sed 's/go\.mod/.../g' | \
		xargs -n 1 go test

build: fmt test
	find . -type f -name go.mod  -exec dirname '{}' ';' | \
		xargs -n 1 go build -C

all: fmt build test
