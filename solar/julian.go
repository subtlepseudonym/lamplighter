package solar

import (
	"time"
)

const (
	EpochJulianDate       = 2440587.5
	TerrestrialTimeOffset = 42.184 // TT offset from unix time
	SecondsPerDay         = 86400  // not including leap seconds
)

// JulianDate returns the Julian date for a particular time,
// including leap seconds
//
// golang does support leap seconds, so they must be added
// Instead, golang uses a leap smear, which is
// how Google production servers handle leap seconds,
// smearing the additional second evenly across 24hrs
// https://developers.google.com/time/smear
func JulianDate(t time.Time) float64 {
	unixWithLeap := float64(t.Unix()) + float64(NumLeapSeconds(t)) + TerrestrialTimeOffset
	return unixWithLeap/SecondsPerDay + EpochJulianDate
}
