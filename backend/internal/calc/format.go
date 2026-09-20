package calc

import (
	"math"
	"strconv"
)

const (
	// Discarding the last few of binary64's ~16 digits absorbs representation error.
	significantDigits = 12

	// The band rendered in decimal rather than scientific notation (ADR-0003).
	decimalUpperBound = 1e12
	decimalLowerBound = 1e-9
)

// Format renders a result for display and is the only place precision or notation is
// decided (ADR-0015). v must be finite; Apply rejects NaN and ±Inf before they reach here.
func Format(v float64) string {
	if v == 0 {
		return "0"
	}

	// Two steps, because a single 'g' call selects scientific below 1e-4 and would
	// contradict the band. 'e' counts digits after the point, so 11 yields 12 significant.
	rounded, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'e', significantDigits-1, 64), 64)

	if magnitude := math.Abs(rounded); magnitude >= decimalUpperBound || magnitude < decimalLowerBound {
		return strconv.FormatFloat(rounded, 'e', -1, 64)
	}

	// -1 is the shortest round-tripping form, which strips trailing zeros.
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
