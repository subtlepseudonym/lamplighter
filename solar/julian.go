package solar

import (
	"time"
)

const (
	JulianEpochDate       = 2451545.0 // Julian date of J2000
	UnixEpochJulianDate   = 2440587.5 // Julian date of the unix epoch
	TerrestrialTimeOffset = 42.184    // TT offset from UTC
	SecondsPerDay         = 86400     // not including leap seconds
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
	return unixWithLeap/SecondsPerDay + UnixEpochJulianDate
}

// JulianDayOfYear calculates the number of Julian days elapsed
// since J2000, the Julian epoch, which occurred on 1 January 2000,
// 12:00 TT (terrestrial time)
func JulianDay(julianDate float64) float64 {
	return julianDate - JulianEpochDate
}
