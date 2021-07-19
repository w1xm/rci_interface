package rotator

import "math"

type Transformer struct {
	Rotator
	latitude     float64
	origCallback StatusCallback
}

type TransformerStatus struct {
	Status
	AzPos, ElPos  float64
	RAPos, DecPos float64
	AzVel, ElVel  float64
}

// equhor converts between azimuth/altitude and hour-angle/declination.
// Phi is the observer's latitude
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

func NewTransformer() *Transformer {
	return &Transformer{}
}

func (t *Transformer) statusCallback(status Status) {
	ra, dec := status.AzimuthPosition(), status.ElevationPosition()

	az, el := equhor_deg(ra, dec, t.latitude)

	t.origCallback(TransformerStatus{
		Status: status,
		AzPos:  az,
		ElPos:  el,
		RAPos:  ra,
		DecPos: dec,
	})
}
