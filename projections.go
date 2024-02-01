package eventsourcing

import "github.com/hallgren/eventsourcing/core"

type Projection struct {
	getF        func() (core.Iterator, error)
	callbackF   func(e Event)
	projections *Projections
}

type Projections struct {
	register     *register // used to map back the event types
	deserializer DeserializeFunc
	projections  []Projection
}

func NewProjections(register *register, deserializer DeserializeFunc) *Projections {
	return &Projections{
		register:     register,
		deserializer: deserializer,
		projections:  make([]Projection, 0),
	}
}

func (p *Projections) Add(getF func() (core.Iterator, error), callbackF func(e Event)) *Projection {
	projection := Projection{
		getF:        getF,
		callbackF:   callbackF,
		projections: p,
	}
	p.projections = append(p.projections, projection)
	return &projection
}

// Close closes all projection
func (p *Projections) Close() {
	for _, projection := range p.projections {
		projection.Close()
	}
}

func (p *Projection) Start() {
	iterator, err := p.getF()
	if err != nil {
		return
	}

	defer iterator.Close()

	for iterator.Next() {
		event, err := iterator.Value()
		if err != nil {
			return
		}

		// TODO: is only registered events of interest?
		f, found := p.projections.register.EventRegistered(event)
		if !found {
			continue
		}

		data := f()
		err = p.projections.deserializer(event.Data, &data)
		if err != nil {
			return
		}
		metadata := make(map[string]interface{})
		err = p.projections.deserializer(event.Metadata, &metadata)
		if err != nil {
			return
		}
		e := NewEvent(event, data, metadata)
		p.callbackF(e)
	}
}

func (p *Projection) Close() {
	// TODO stop the projection
}
