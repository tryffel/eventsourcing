package memory

import "github.com/hallgren/eventsourcing/core"

type iterator struct {
	events   []core.Event
	position int
	event    core.Event
}

func (i *iterator) Next() bool {
	if len(i.events) <= i.position {
		return false
	}
	i.event = i.events[i.position]
	i.position++
	return true
}

func (i *iterator) Value() (core.Event, error) {
	return i.event, nil
}

func (i *iterator) Close() {
	i.events = nil
	i.position = 0
}
