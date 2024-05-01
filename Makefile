all:
	go fmt ./...
	go build

	#core
	cd core && go build
	# event stores
	cd eventstore/bbolt && go build
	cd eventstore/sql && go build
	cd eventstore/esdb && go build
test:
	#core
	cd core && go test -count 1 ./...
	# event stores
	cd eventstore/bbolt && go test -count 1 ./...
	cd eventstore/sql && go test -count 1 ./...
	cd eventstore/esdb && go test esdb_test.go -count 1 ./...

	# main
	go test -count 1 ./...
update:
	# event stores
	cd eventstore/bbolt && go get -t -u ./... && go mod tidy
	cd eventstore/sql && go get -u ./... && go mod tidy
	cd eventstore/esdb && go get -t -u ./... && go mod tidy

	#snaptshot stores
	cd snapshotstore/sql && go get -u ./... && go mod tidy
 
	# main
	go get -t -u ./... && go mod tidy
