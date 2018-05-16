package rci

import (
	"context"
	"sync"
)

type Offset struct {
	*RCI
	mu sync.Mutex
	// last set positions (without offset)
	az, el float64
	// in position mode
	azFlags, elFlags string
	// offsetAz and offsetEl are added to the returned position and subtracted from requested positions.
	offsetAz, offsetEl float64
}

func add(angle, offset float64) float64 {
	angle += offset
	for angle >= 360 {
		angle -= 360
	}
	for angle < 0 {
		angle += 360
	}
	return angle
}

func ConnectOffset(ctx context.Context, port string, statusCallback StatusCallback, offsetAz, offsetEl float64) (*Offset, error) {
	o := &Offset{}
	cb := func(status Status) {
		o.mu.Lock()
		status.AzPos = add(status.AzPos, o.offsetAz)
		status.ElPos = add(status.ElPos, o.offsetEl)
		status.CommandAzPos = add(status.CommandAzPos, o.offsetAz)
		status.CommandElPos = add(status.CommandElPos, o.offsetEl)
		o.azFlags = status.CommandAzFlags
		o.elFlags = status.CommandElFlags
		o.mu.Unlock()
		statusCallback(status)
	}
	rci, err := Connect(ctx, port, cb)
	if err != nil {
		return nil, err
	}
	o.RCI = rci
	return o, nil
}

func (o *Offset) SetAzimuthOffset(offset float64) {
	o.mu.Lock()
	o.offsetAz = offset
	do := o.azFlags == "POSITION"
	o.mu.Unlock()
	if do {
		o.RCI.SetAzimuthPosition(add(o.az, -offset))
	}
}

func (o *Offset) SetElevationOffset(offset float64) {
	o.mu.Lock()
	o.offsetEl = offset
	do := o.elFlags == "POSITION"
	o.mu.Unlock()
	if do {
		o.RCI.SetElevationPosition(add(o.el, -offset))
	}
}

func (o *Offset) SetAzimuthPosition(position float64) {
	o.mu.Lock()
	o.az = position
	offset := o.offsetAz
	o.mu.Unlock()
	o.RCI.SetAzimuthPosition(add(o.az, -offset))
}

func (o *Offset) SetElevationPosition(position float64) {
	o.mu.Lock()
	o.el = position
	offset := o.offsetEl
	o.mu.Unlock()
	o.RCI.SetElevationPosition(add(o.el, -offset))
}
