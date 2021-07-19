package status

import (
	"errors"
	"strconv"
	"strings"

	"github.com/w1xm/rci_interface/rotator"
)

type Status struct {
	// \?ENC command returns:
	RawAzPos int32
	RawElPos int32
	RawAzVel int32
	RawElVel int32

	// AZ command returns:
	AzPos float64 `report:"AZ"`
	// EL command returns:
	ElPos float64 `report:"EL"`

	// IP0 returns temperature
	Temperature float64 `report:"IP0,"`

	// IP1 returns Az endstop
	AzimuthCCW, AzimuthCW bool
	// IP2 returns El endstop
	ElevationLower, ElevationUpper bool

	// IP3 returns Az position (redundant)
	// IP4 returns El position (redundant)

	// IP5 returns Az drive load
	RawAzDrive float64
	// IP6 returns El drive load
	RawElDrive float64
	// IP7 returns Az speed
	AzVel float64 `report:"IP7,"`
	// IP8 returns El speed
	ElVel float64 `report:"IP8,"`

	// CR1-3 are azimuth P-I-D
	// CR4-6 are elevation P-I-D
	// CR7 is azimuth park position
	// CR8 is elevation park position
	// CR10-13 return Az position setpoint, El position setpoint, Az velocity setpoint, El velocity setpoint (not supported by SatNOGS)
	CommandAzPos float64 `report:"CR10,"`
	CommandElPos float64 `report:"CR11,"`
	CommandAzVel float64 `report:"CR12,"`
	CommandElVel float64 `report:"CR13,"`

	// GS command returns:
	StatusRegister uint64 `report:"GS"`
	// GE command returns
	ErrorRegister uint64 `report:"GE"`
	ErrorFlags    struct {
		NoError     bool
		SensorError bool
		HomingError bool
		MotorError  bool
	}

	// VE command returns:
	Version string `report:"VE"`

	HostOkay bool

	Moving bool

	CommandAzFlags, CommandElFlags string
}

func (s Status) Clone() rotator.Status {
	return s
}

func (s Status) AzimuthPosition() float64 {
	return s.AzPos
}

func (s Status) ElevationPosition() float64 {
	return s.ElPos
}

func ParseFloat(dest *float64, input string) error {
	f, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return err
	}
	*dest = f
	return nil
}

func ParseFloatArray(dest []*float64, input string) error {
	parts := strings.Split(input, ",")
	for i, field := range dest {
		if i >= len(parts) {
			return errors.New("truncated list")
		}
		if err := ParseFloat(field, parts[i]); err != nil {
			return err
		}
	}
	return nil
}
