package eventsourcing

import (
	"errors"
	"reflect"
)

type eventFunc = func() interface{}
type EventsFunc = func(events ...interface{}) error

type register struct {
	r map[string]eventFunc
}

type aggregate interface {
	RegisterEvents(EventsFunc) error
}

var (
	// ErrAggregateNameMissing return if aggregate name is missing
	ErrAggregateNameMissing = errors.New("missing aggregate name")

	// ErrNoEventsToRegister return if no events to register
	ErrNoEventsToRegister = errors.New("no events to register")

	// ErrEventNameMissing return if Event name is missing
	ErrEventNameMissing = errors.New("missing event name")
)

func newRegister() *register {
	return &register{
		r: make(map[string]eventFunc),
	}
}

// Type return the func to generate the correct event data type
func (r *register) Type(typ, reason string) (eventFunc, bool) {
	d, ok := r.r[typ+"_"+reason]
	return d, ok
}

func (r *register) RegisterAggregate(a aggregate) error {
	typ := reflect.TypeOf(a).Elem().Name()
	if typ == "" {
		return ErrAggregateNameMissing
	}

	// fe is a helper function to make the event type registration simpler
	fe := func(events ...interface{}) []eventFunc {
		res := []eventFunc{}
		for _, e := range events {
			res = append(res, eventToFunc(e))
		}
		return res
	}

	fu := func(events ...interface{}) error {
		eventsF := fe(events...)
		for _, f := range eventsF {
			event := f()
			reason := reflect.TypeOf(event).Elem().Name()
			if reason == "" {
				return ErrEventNameMissing
			}
			r.r[typ+"_"+reason] = f
		}
		return nil
	}

	return a.RegisterEvents(fu)
}

func eventToFunc(event interface{}) eventFunc {
	return func() interface{} { return event }
}
