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
	register *Register // used to map the event types
	Encoder  encoder
	count    int
}

func NewProjectionHandler(register *Register, encoder encoder) *ProjectionHandler {
	return &ProjectionHandler{
		register: register,
		Encoder:  encoder,
	}
}

type Projection struct {
	fetchF    fetchFunc
	callbackF callbackFunc
	handler   *ProjectionHandler
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
	ErrChan     chan ProjectionResult
}

// ProjectionResult is the return type for a Group and Race
type ProjectionResult struct {
	Error error
	Name  string
	Event Event
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
func (p *Projection) Run(ctx context.Context) ProjectionResult {
	var result ProjectionResult
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ProjectionResult{Error: ctx.Err(), Name: result.Name, Event: result.Event}
		case <-timer.C:
			result = p.RunToEnd(ctx)
			if result.Error != nil {
				return result
			}
		}
		// rest
		timer.Reset(p.Pace)
	}
}

// RunToEnd runs until the projection reaches the end of the event stream
func (p *Projection) RunToEnd(ctx context.Context) ProjectionResult {
	var result ProjectionResult
	for {
		select {
		case <-ctx.Done():
			return ProjectionResult{Error: ctx.Err(), Name: result.Name, Event: result.Event}
		default:
			ran, result := p.RunOnce()
			if result.Error != nil {
				return result
			}
			// hit the end of the event stream
			if !ran {
				return result
			}
		}
	}
}

// RunOnce runs the fetch method one time
func (p *Projection) RunOnce() (bool, ProjectionResult) {
	// ran indicate if there were events to fetch
	var ran bool
	var eventRecord Event

	iterator, err := p.fetchF()
	if err != nil {
		return false, ProjectionResult{Error: err, Name: p.Name, Event: eventRecord}
	}
	defer iterator.Close()

	for iterator.Next() {
		ran = true
		event, err := iterator.Value()
		if err != nil {
			return false, ProjectionResult{Error: err, Name: p.Name, Event: eventRecord}
		}

		// TODO: is only registered events of interest?
		f, found := p.handler.register.EventRegistered(event)
		if !found {
			if p.Strict {
				err = fmt.Errorf("event not registered aggregate type: %s, reason: %s, global version: %d, %w", event.AggregateType, event.Reason, event.GlobalVersion, ErrEventNotRegistered)
				return false, ProjectionResult{Error: err, Name: p.Name, Event: eventRecord}
			}
			continue
		}

		data := f()
		err = p.handler.Encoder.Deserialize(event.Data, &data)
		if err != nil {
			return false, ProjectionResult{Error: err, Name: p.Name, Event: eventRecord}
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = p.handler.Encoder.Deserialize(event.Metadata, &metadata)
			if err != nil {
				return false, ProjectionResult{Error: err, Name: p.Name, Event: eventRecord}
			}
		}
		e := NewEvent(event, data, metadata)

		// keep a reference to the event currently processing
		eventRecord = e
		err = p.callbackF(e)
		if err != nil {
			return false, ProjectionResult{Error: err, Name: p.Name, Event: eventRecord}
		}
	}
	return ran, ProjectionResult{Error: nil, Name: p.Name, Event: eventRecord}
}

// Group runs a group of projections concurrently
func (ph *ProjectionHandler) Group(projections ...*Projection) *Group {
	return &Group{
		handler:     ph,
		projections: projections,
		cancelF:     func() {},
		ErrChan:     make(chan ProjectionResult),
	}
}

// Start starts all projectinos in the group and return a channel to notify if a errors is returned from a projection
func (g *Group) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelF = cancel

	g.wg.Add(len(g.projections))
	for _, projection := range g.projections {
		go func(p *Projection) {
			defer g.wg.Done()
			result := p.Run(ctx)
			if !errors.Is(result.Error, context.Canceled) {
				g.ErrChan <- result
			}
		}(projection)
	}
}

// Stop terminate all projections in the group
func (g *Group) Stop() {
	g.cancelF()

	// return when all projections has stopped
	g.wg.Wait()

	// close the error channel
	close(g.ErrChan)
}

// Race runs the projections to the end of the events streams.
// Can be used on a stale event stream with no more events coming in or when you want to know when all projections are done.
func (p *ProjectionHandler) Race(cancelOnError bool, projections ...*Projection) ([]ProjectionResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(len(projections))

	var lock sync.Mutex
	var causingErr error

	results := make([]ProjectionResult, len(projections))
	for i, projection := range projections {
		go func(pr *Projection, index int) {
			defer wg.Done()
			result := pr.RunToEnd(ctx)
			if result.Error != nil {
				if !errors.Is(result.Error, context.Canceled) && cancelOnError {
					cancel()

					lock.Lock()
					causingErr = result.Error
					lock.Unlock()
				}
			}
			lock.Lock()
			results[index] = result
			lock.Unlock()
		}(projection, i)
	}
	wg.Wait()
	return results, causingErr
}
