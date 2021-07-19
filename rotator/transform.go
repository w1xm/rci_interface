package rotator

import (
	"log"
	"math"
	"sync"
)

type Transformer struct {
	Rotator
	latitude     float64
	origCallback StatusCallback
	mu           sync.Mutex
	status       TransformerStatus
}

type TransformerStatus struct {
	Status
	AzPos, ElPos                   float64
	LhaPos, DecPos                 float64
	AzVel, ElVel                   float64
	CommandAzPos, CommandElPos     float64
	CommandAzFlags, CommandElFlags string
}

func (ts TransformerStatus) Clone() Status {
	ts.Status = ts.Status.Clone()
	return ts
}

func (ts TransformerStatus) AzimuthPosition() float64 {
	return ts.AzPos
}

func (ts TransformerStatus) ElevationPosition() float64 {
	return ts.AzPos
}

// equhor converts between azimuth/altitude and hour-angle/declination.
// phi is the observer's latitude
// Arguments are in radians
// Algorithm from https://metacpan.org/dist/Astro-Montenbruck/source/lib/Astro/Montenbruck/CoCo.pm
func equhor_rad(x, y, phi float64) (float64, float64) {
	sx, sy, sphi := math.Sin(x), math.Sin(y), math.Sin(phi)
	cx, cy, cphi := math.Cos(x), math.Cos(y), math.Cos(phi)

	sq := (sy * sphi) + (cy * cphi * cx)
	q := math.Asin(sq)

	cp := (sy - (sphi * sq)) / (cphi * math.Cos(q))
	p := math.Acos(cp)
	if sx > 0 {
		p = 2*math.Pi - p
	}
	return p, q
}

func deg2rad(x float64) float64 {
	return x * math.Pi / 180
}

func rad2deg(x float64) float64 {
	return x * 180 / math.Pi
}

func equhor_deg(x, y, phi float64) (float64, float64) {
	x, y, phi = deg2rad(x), deg2rad(y), deg2rad(phi)
	p, q := equhor_rad(x, y, phi)
	return rad2deg(p), rad2deg(q)
}

// func hor2equ(az, el, phi float64) (float64, float64) {
// 	h, dec := equhor_deg(az+180, el, phi)
// 	return math.Mod(360-h, 360), dec
// }

func hor2equ(az, el, phi float64) (float64, float64) {
	sinA := math.Sin(deg2rad(az))
	cosA := math.Cos(deg2rad(az))
	sinE := math.Sin(deg2rad(el))
	cosE := math.Cos(deg2rad(el))
	sinL := math.Sin(deg2rad(phi))
	cosL := math.Cos(deg2rad(phi))

	x := -cosA*cosE*sinL + sinE*cosL
	y := -sinA * cosE
	z := cosA*cosE*cosL + sinE*sinL

	r := math.Sqrt(x*x + y*y)
	ha := 0.0
	if r != 0 {
		ha = math.Atan2(y, x)
	}
	dec := math.Atan2(z, r)

	return rad2deg(ha), rad2deg(dec)
}

func NewTransformer(latitude float64, constructor func(cb StatusCallback) (Rotator, error), cb StatusCallback) (*Transformer, error) {
	t := &Transformer{
		origCallback: cb,
	}
	r, err := constructor(t.statusCallback)
	if err != nil {
		return nil, err
	}
	t.Rotator = r
	return t, nil
}

func (t *Transformer) SetAzimuthPosition(az float64) {
	t.mu.Lock()
	el := t.status.ElPos
	if t.status.CommandElFlags == "POSITION" {
		el = t.status.CommandElPos
	}
	t.mu.Unlock()

	lha, dec := hor2equ(az, el, t.latitude)

	log.Printf("SetAzimuthPosition: (%3.2f, %3.2f) -> (%3.2f, %3.2f)", az, el, lha, dec)

	t.Rotator.SetAzimuthPosition(lha)
	t.Rotator.SetElevationPosition(dec)
}

func (t *Transformer) SetElevationPosition(el float64) {
	t.mu.Lock()
	t.status.CommandElFlags = "POSITION"
	t.status.CommandElPos = el
	az := t.status.AzPos
	if t.status.CommandAzFlags == "POSITION" {
		el = t.status.CommandAzPos
	}
	t.mu.Unlock()

	lha, dec := hor2equ(az, el, t.latitude)

	log.Printf("SetElevationPosition: (%3.2f, %3.2f) -> (%3.2f, %3.2f)", az, el, lha, dec)

	t.Rotator.SetAzimuthPosition(lha)
	t.Rotator.SetElevationPosition(dec)
}

func (t *Transformer) statusCallback(status Status) {
	lha, dec := status.AzimuthPosition(), status.ElevationPosition()

	az, el := equhor_deg(lha, dec, t.latitude)

	lhaflags, lhacmd := status.AzimuthCommand()
	decflags, deccmd := status.ElevationCommand()

	flags := "NONE"
	if lhaflags == "POSITION" || decflags == "POSITION" {
		flags = "POSITION"
	}

	azcmd, elcmd := equhor_deg(lhacmd, deccmd, t.latitude)

	ts := TransformerStatus{
		Status:         status,
		AzPos:          az,
		ElPos:          el,
		LhaPos:         lha,
		DecPos:         dec,
		CommandAzFlags: flags,
		CommandElFlags: flags,
		CommandAzPos:   azcmd,
		CommandElPos:   elcmd,
	}
	t.mu.Lock()
	t.status = ts
	t.mu.Unlock()
	t.origCallback(ts)
}
