module github.com/hallgren/eventsourcing/eventstore/bbolt

go 1.22

toolchain go1.22.6

require (
	github.com/hallgren/eventsourcing/core v0.4.0
	go.etcd.io/bbolt v1.3.11
)

require golang.org/x/sys v0.25.0 // indirect

// replace github.com/hallgren/eventsourcing/core => ../../core
