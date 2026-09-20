package calc

import (
	"math"
	"strconv"
)

const (
	// Discarding the last few of binary64's ~15.95 decimal digits absorbs representation
	// error without costing precision a display can show.
	significantDigits = 12

	// The band written in decimal rather than scientific notation (ADR-0003 clause 4).
	decimalUpperBound = 1e12
	decimalLowerBound = 1e-9
)

// Format renders a result in its canonical display form: 12 significant digits,
// round-half-to-even, decimal inside [1e-9, 1e12) and scientific outside it, trailing zeros
// stripped and negative zero normalized. It is the only place precision or notation is
// decided (ADR-0015).
//
// v must be finite; Apply rejects NaN and ±Inf before they reach here.
func Format(v float64) string {
	if v == 0 {
		return "0"
	}

	// Rounding and notation are two steps: a single 'g' call would select scientific for
	// everything below 1e-4 and contradict the band. 'e' counts digits after the point, so
	// precision 11 yields 12 significant digits at any magnitude.
	rounded, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'e', significantDigits-1, 64), 64)

	// Notation follows the rounded value, so a value that rounds across a band edge is
	// rendered in the band it lands in.
	if magnitude := math.Abs(rounded); magnitude >= decimalUpperBound || magnitude < decimalLowerBound {
		return strconv.FormatFloat(rounded, 'e', -1, 64)
	}

	// Precision -1 emits the shortest round-tripping form, stripping trailing zeros.
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
