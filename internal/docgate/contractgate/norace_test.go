//go:build !race

package contractgate

// raceDetectorEnabled is false in ordinary and coverage builds, which record
// the full live-corpus measurements without any wall-clock assertion.
const raceDetectorEnabled = false
