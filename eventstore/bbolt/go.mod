module github.com/hallgren/eventsourcing/eventstore/bbolt

go 1.13

require (
	github.com/hallgren/eventsourcing/core v0.4.0
	go.etcd.io/bbolt v1.3.8
	golang.org/x/sys v0.17.0 // indirect
)

// replace github.com/hallgren/eventsourcing/core => ../../core
