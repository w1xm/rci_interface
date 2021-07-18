package rotator

type Rotator interface {
	Stop()
	SetAzimuthPosition(angle float64)
	SetElevationPosition(angle float64)
	SetAzimuthVelocity(angle float64)
	SetElevationVelocity(angle float64)
	// SetAcceptableShutdowns(value map[uint8]bool)
	// Write(register int, values ...uint16)
	// SetMovingDisabled(blocked bool)
	// ExitShutdown()
}

type StatusCallback func(status Status)

type Status interface {
	AzimuthPosition() float64
	ElevationPosition() float64

	Clone() Status
}

type Shutdowner interface {
	ExitShutdown()
	SetAcceptableShutdowns(map[uint8]bool)
}

type Offsetter interface {
	SetAzimuthOffset(offset float64)
	SetElevationOffset(offset float64)
}

type SetMovingDisableder interface {
	SetMovingDisabled(bool)
}

type Writer interface {
	Write(register int, values ...uint16)
}
