//go:build race

package contractgate

// raceDetectorEnabled limits redundant live-corpus measurements under -race.
// Timing is advisory in every mode; fixture correctness tests still run.
const raceDetectorEnabled = true
