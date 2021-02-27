package solar

import (
	"math"
)

// MeanSolarNoon approximates solar noon for the mean sun
// for a given Julian day at a given longitude.
// For the purpose of this calculation, longitude is degrees
// west, with negative values for degrees east.
func MeanSolarNoon(julianDay, longitudeWest float64) float64 {
	return julianDay - longitudeWest/360
}

// SolarMeanAnomaly calculates the fraction of the sun's
// orbital period elapsed since perihelion.
func SolarMeanAnomaly(meanSolarNoon float64) float64 {
	return math.Mod(357.5291 + (0.98560028 * meanSolarNoon), 360)
}

// EquationOfTheCenter calculates the angular difference
// between the position of the actual sun (with an elliptical
// orbit) and the mean sun (with a circular orbit). This
// can be expressed as a function of mean anomaly and 
// orbital eccentricity.
//
// https://en.wikipedia.org/wiki/Equation_of_the_center
func EquationOfTheCenter(meanAnomaly float64) float64 {
	firstOrder := 1.9148 * math.Sin(meanAnomaly)
	secondOrder := 0.02 * math.Sin(2 * meanAnomaly)
	thirdOrder := 0.0003 * math.Sin(3 * meanAnomaly)

	return firstOrder + secondOrder + thirdOrder
}

// EclipticLongitude calculates the sun's distance along the
// ecliptic
func EclipticLongitude(meanAnomaly, center float64) float64 {
	return 0
}
