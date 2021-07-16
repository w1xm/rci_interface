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
