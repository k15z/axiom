package runner

const defaultConcurrency = 5

// AutoConcurrency returns a reasonable concurrency level based on the number
// of tests to run. Returns min(defaultConcurrency, numTests), minimum 1.
func AutoConcurrency(numTests int) int {
	if numTests <= 1 {
		return 1
	}
	if numTests < defaultConcurrency {
		return numTests
	}
	return defaultConcurrency
}
