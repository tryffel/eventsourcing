package eventsourcing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hallgren/eventsourcing/core"
)

type fetchFunc func() (core.Iterator, error)
type callbackFunc func(e Event) error

type ProjectionHandler struct {
	Register     *Register // used to map the event types
	Deserializer DeserializeFunc
	count        int
}

type Projection struct {
	fetchF    fetchFunc
	callbackF callbackFunc
	handler   *ProjectionHandler
	event     Event
	Pace      time.Duration // Pace is used when a projection is running and it reaches the end of the event stream
	Strict    bool          // Strict indicate if the projection should return error if the event it fetches is not found in the regiter
	Name      string
}

// Group runs projections concurrently
type Group struct {
	handler     *ProjectionHandler
	projections []*Projection
	cancelF     context.CancelFunc
	wg          sync.WaitGroup
}

// Projection creates a projection that will run down an event stream
func (ph *ProjectionHandler) Projection(fetchF fetchFunc, callbackF callbackFunc) *Projection {
	projection := Projection{
		fetchF:    fetchF,
		callbackF: callbackF,
		handler:   ph,
		Pace:      time.Second * 10,            // Default pace 10 seconds
		Strict:    true,                        // Default strict is active
		Name:      fmt.Sprintf("%d", ph.count), // Default the name to it's creation index
	}
	ph.count++
	return &projection
}

// Run runs the projection forever until the context is cancelled. When there are no more events to consume it
// sleeps the set pace before it runs again.
func (p *Projection) Run(ctx context.Context) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			err := p.RunToEnd(ctx)
			if err != nil {
				return err
			}
		}
		// rest
		timer.Reset(p.Pace)
	}
}

// RunToEnd runs until the projection reaches the end of the event stream
func (p *Projection) RunToEnd(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			ran, err := p.RunOnce()
			if err != nil {
				return err
			}
			// hit the end of the event stream
			if !ran {
				return nil
			}
		}
	}
}

// RunOnce runs the fetch method one time
func (p *Projection) RunOnce() (bool, error) {
	iterator, err := p.fetchF()
	if err != nil {
		return false, err
	}
	defer iterator.Close()

	// ran indicate if there were events to fetch
	var ran bool
	for iterator.Next() {
		ran = true
		event, err := iterator.Value()
		if err != nil {
			return false, err
		}

		// TODO: is only registered events of interest?
		f, found := p.handler.Register.EventRegistered(event)
		if !found {
			if p.Strict {
				return false, fmt.Errorf("event not registered aggregate type: %s, reason: %s, global version: %d, %w", event.AggregateType, event.Reason, event.GlobalVersion, ErrEventNotRegistered)
			}
			continue
		}

		data := f()
		err = p.handler.Deserializer(event.Data, &data)
		if err != nil {
			return false, err
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = p.handler.Deserializer(event.Metadata, &metadata)
			if err != nil {
				return false, err
			}
		}
		e := NewEvent(event, data, metadata)

		// keep a reference to the event currently processing
		p.event = e
		err = p.callbackF(e)
		if err != nil {
			return false, err
		}
	}
	return ran, nil
}

// Group runs a group of projections concurrently
func (ph *ProjectionHandler) Group(projections ...*Projection) *Group {
	return &Group{
		handler:     ph,
		projections: projections,
		cancelF:     func() {},
	}
}

// Start starts all projectinos in the group and return a channel to notify if a errors is returned from a projection
func (g *Group) Start() chan error {
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelF = cancel

	g.wg.Add(len(g.projections))
	for _, projection := range g.projections {
		go func(p *Projection) {
			defer g.wg.Done()
			err := p.Run(ctx)
			if !errors.Is(err, context.Canceled) {
				errChan <- err
			}
		}(projection)
	}
	return errChan
}

// Close stops all projections in the group
func (g *Group) Close() {
	g.cancelF()

	// return when all projections has stopped
	g.wg.Wait()
}

type RaceResult struct {
	Error          error
	ProjectionName string
	Event          Event
}

// Race runs the projections to the end of the there events streams.
// Can be used on a stale event stream with now more events comming in or when you want to know when all projections are done.
func (p *ProjectionHandler) Race(cancelOnError bool, projections ...*Projection) ([]RaceResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(len(projections))

	var lock sync.Mutex
	var causingErr error

	result := make([]RaceResult, len(projections))
	for i, projection := range projections {
		go func(pr *Projection, index int) {
			defer wg.Done()
			err := pr.RunToEnd(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) && cancelOnError {
					cancel()

					lock.Lock()
					causingErr = err
					lock.Unlock()
				}
			}
			lock.Lock()
			result[index] = RaceResult{Error: err, ProjectionName: pr.Name, Event: pr.event}
			lock.Unlock()
		}(projection, i)
	}
	wg.Wait()
	return result, causingErr
}
