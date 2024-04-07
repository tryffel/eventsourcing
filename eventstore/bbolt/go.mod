module github.com/hallgren/eventsourcing/eventstore/bbolt

go 1.17

require (
	github.com/hallgren/eventsourcing/core v0.4.0
	go.etcd.io/bbolt v1.3.9
)

require golang.org/x/sys v0.19.0 // indirect

// replace github.com/hallgren/eventsourcing/core => ../../core
