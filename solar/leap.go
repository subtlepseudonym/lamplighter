package solar

import (
	"time"
	"sort"
)

// leapSeconds is the list of time values to the nanosecond
// after which a leap second was added
//
// Values are taken from this page on 29 June, 2020:
// https://www.ietf.org/timezones/data/leap-seconds.list
// 
// The number of leap seconds to add to the calculated
// Julian date can be determined by how many of these
// values are lower than the unix time being converted.
var leapSeconds = []time.Time{
	getLeapSecondTime(1972, time.June, 30),
	getLeapSecondTime(1972, time.December, 31),
	getLeapSecondTime(1973, time.December, 31),
	getLeapSecondTime(1974, time.December, 31),
	getLeapSecondTime(1975, time.December, 31),
	getLeapSecondTime(1976, time.December, 31),
	getLeapSecondTime(1977, time.December, 31),
	getLeapSecondTime(1978, time.December, 31),
	getLeapSecondTime(1979, time.December, 31),
	getLeapSecondTime(1981, time.June, 30),
	getLeapSecondTime(1982, time.June, 30),
	getLeapSecondTime(1983, time.June, 30),
	getLeapSecondTime(1985, time.June, 30),
	getLeapSecondTime(1987, time.December, 31),
	getLeapSecondTime(1989, time.December, 31),
	getLeapSecondTime(1990, time.December, 31),
	getLeapSecondTime(1992, time.June, 30),
	getLeapSecondTime(1993, time.June, 30),
	getLeapSecondTime(1994, time.June, 30),
	getLeapSecondTime(1995, time.December, 31),
	getLeapSecondTime(1997, time.June, 30),
	getLeapSecondTime(1998, time.December, 31),
	getLeapSecondTime(2005, time.December, 31),
	getLeapSecondTime(2008, time.December, 31),
	getLeapSecondTime(2012, time.June, 30),
	getLeapSecondTime(2015, time.June, 30),
	getLeapSecondTime(2016, time.December, 31),
}

// getLeapSecondTime is shorthand for the time.Time representing the
// final nanosecond before a leap second is added.
func getLeapSecondTime(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 23, 59, 59, 999999999, time.UTC)
}

// NumLeapSeconds calculates the number of leap seconds
// that have occurred between 1 July 1972 and t.
//
// As of go1.14, leap seconds are not supported by the
// time package. This may change in go2
// https://github.com/golang/go/issues/15247
//
// Demonstration of the time package missing leap seconds:
// https://play.golang.org/p/EcH0-dzh9w1
func NumLeapSeconds(t time.Time) int64 {
	idx := sort.Search(len(leapSeconds), func(i int) bool {
		return t.Before(leapSeconds[i])
	})

	return int64(idx + 1)
}
