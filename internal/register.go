package internal

import (
	"reflect"

	"github.com/hallgren/eventsourcing/core"
)

type registerFunc = func() interface{}

type Register struct {
	aggregateEvents map[string]registerFunc
	aggregates      map[string]struct{}
}

// Aggregate interface to use the aggregate root specific methods
type aggregate interface {
	Register(func(events ...interface{}))
}

func NewRegister() *Register {
	return &Register{
		aggregateEvents: make(map[string]registerFunc),
		aggregates:      make(map[string]struct{}),
	}
}

// EventRegistered return the func to generate the correct event data type and true if it exists
// otherwise false.
func (r *Register) EventRegistered(event core.Event) (registerFunc, bool) {
	d, ok := r.aggregateEvents[event.AggregateType+"_"+event.Reason]
	return d, ok
}

// AggregateRegistered return true if the aggregate is registered
func (r *Register) AggregateRegistered(a aggregate) bool {
	typ := AggregateType(a)
	_, ok := r.aggregates[typ]
	return ok
}

// Register store the aggregate and calls the aggregate method Register to register the aggregate events.
func (r *Register) Register(a aggregate) {
	typ := AggregateType(a)
	r.aggregates[typ] = struct{}{}

	// fe is a helper function to make the event type registration simpler
	fe := func(events ...interface{}) []registerFunc {
		res := []registerFunc{}
		for _, e := range events {
			res = append(res, eventToFunc(e))
		}
		return res
	}

	fu := func(events ...interface{}) {
		eventsF := fe(events...)
		for _, f := range eventsF {
			event := f()
			reason := reflect.TypeOf(event).Elem().Name()
			r.aggregateEvents[typ+"_"+reason] = f
		}
	}
	a.Register(fu)
}

func eventToFunc(event interface{}) registerFunc {
	return func() interface{} { return event }
}

func AggregateType(a interface{}) string {
	return reflect.TypeOf(a).Elem().Name()
}
