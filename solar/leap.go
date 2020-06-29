package solar

import (
	"time"
	"sort"
)

// leapSeconds is the list of seconds after
// which a leap second was added
var leapSeconds []int64{
	78796799,
	94694399,
	126230399,
	157766399,
	189302399,
	220924799,
	252460799,
	283996799,
	315532799,
	362793599,
	394329599,
	425865599,
	489023999,
	567993599,
	631151999,
	662687999,
	709948799,
	741484799,
	773020799,
	820454399,
	867715199,
	915148799,
	1136073599,
	1230767999,
	1341100799,
	1435708799,
	1483228799,
}

// NumLeapSeconds calculates the number of leap seconds
// that have occurred between 1 July 1972 and t.
func NumLeapSeconds(t time.Time) {
	u := t.Unix()
	idx := sort.Search(len(leapSeconds), func(i int) bool {
		return u > leapSeconds[i]
	})

	return idx + 1
}

// calculateLeapSeconds is included purely to indicate
// the method by which the "magic" integer values in
// leapSeconds were obtained. Each value is
// the unix time (in seconds) after which a leap second
// was added.
//
// Values are taken from this page on 29 June, 2020:
// https://www.ietf.org/timezones/data/leap-seconds.list
// 
// The number of leap seconds to add to the calculated
// Julian date can be determined by how many of these
// values are lower than the unix time being converted.
func calculateLeapSeconds() []int64 {
	return []int64{
		getLeapSecondTime(1972, time.June, 30).Unix(),
		getLeapSecondTime(1972, time.December, 31).Unix(),
		getLeapSecondTime(1973, time.December, 31).Unix(),
		getLeapSecondTime(1974, time.December, 31).Unix(),
		getLeapSecondTime(1975, time.December, 31).Unix(),
		getLeapSecondTime(1976, time.December, 31).Unix(),
		getLeapSecondTime(1977, time.December, 31).Unix(),
		getLeapSecondTime(1978, time.December, 31).Unix(),
		getLeapSecondTime(1979, time.December, 31).Unix(),
		getLeapSecondTime(1981, time.June, 30).Unix(),
		getLeapSecondTime(1982, time.June, 30).Unix(),
		getLeapSecondTime(1983, time.June, 30).Unix(),
		getLeapSecondTime(1985, time.June, 30).Unix(),
		getLeapSecondTime(1987, time.December, 31).Unix(),
		getLeapSecondTime(1989, time.December, 31).Unix(),
		getLeapSecondTime(1990, time.December, 31).Unix(),
		getLeapSecondTime(1992, time.June, 30).Unix(),
		getLeapSecondTime(1993, time.June, 30).Unix(),
		getLeapSecondTime(1994, time.June, 30).Unix(),
		getLeapSecondTime(1995, time.December, 31).Unix(),
		getLeapSecondTime(1997, time.June, 30).Unix(),
		getLeapSecondTime(1998, time.December, 31).Unix(),
		getLeapSecondTime(2005, time.December, 31).Unix(),
		getLeapSecondTime(2008, time.December, 31).Unix(),
		getLeapSecondTime(2012, time.June, 30).Unix(),
		getLeapSecondTime(2015, time.June, 30).Unix(),
		getLeapSecondTime(2016, time.December, 31).Unix(),
	}
}

// getLeapSecondTime is shorthand for the time.Time representing the
// final nanosecond before a leap second is added.
func getLeapSecondTime(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 23, 59, 59, 999999999, time.UTC)
}
