package calc

import (
	"math"
	"strconv"
)

const (
	// significantDigits discards roughly the last four of binary64's ~15.95 decimal
	// digits, which absorbs accumulated representation error without costing precision a
	// display can show (ADR-0003 clause 2).
	significantDigits = 12

	// The band within which results are written in decimal rather than scientific
	// notation (ADR-0003 clause 4).
	decimalUpperBound = 1e12
	decimalLowerBound = 1e-9
)

// Format renders a result in its canonical display form: rounded to 12 significant digits,
// round-half-to-even, decimal inside [1e-9, 1e12) and scientific outside it, trailing zeros
// stripped and negative zero normalized.
//
// This is the only place precision or notation is decided (ADR-0015). Rounding and notation
// are deliberately two steps: a single 'g' format call would select scientific notation for
// everything below 1e-4 and contradict the band.
//
// v must be finite. Apply rejects NaN and ±Inf before they reach here, which is what keeps
// a non-finite value off the wire (ADR-0015 step 1).
func Format(v float64) string {
	if v == 0 {
		return "0"
	}

	// 'e' counts digits after the point, so precision 11 yields 12 significant digits at
	// any magnitude. Re-parsing produces the rounded binary64 value.
	// The error is unreachable: the input is a literal FormatFloat just produced from a
	// finite float64, so it always parses and is always in range.
	rounded, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'e', significantDigits-1, 64), 64)

	// Notation follows the rounded value, not the original: a value that rounds across a
	// band edge is rendered in the band it lands in.
	if magnitude := math.Abs(rounded); magnitude >= decimalUpperBound || magnitude < decimalLowerBound {
		return strconv.FormatFloat(rounded, 'e', -1, 64)
	}

	// Precision -1 emits the shortest form that round-trips, which strips trailing zeros
	// and cannot exceed 12 significant digits, since the value already round-trips from a
	// 12-digit decimal.
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
