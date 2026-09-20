package calc

import (
	"math"
	"strconv"
	"strings"
)

// num parses a float64 at runtime. Every value in this package's tables is written as a
// string and passed through here: Go folds untyped constant expressions in arbitrary
// precision, so a table written as `0.1 + 0.2` would be exactly 0.3 at compile time and the
// test would never exercise float64.
func num(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic("test table holds a value that is not a float64 literal: " + s)
	}
	return v
}

func nums(ss []string) []float64 {
	out := make([]float64, len(ss))
	for i, s := range ss {
		out[i] = num(s)
	}
	return out
}

// identical compares bit patterns, so -0 is distinguishable from 0. The arithmetic layer
// returns the raw IEEE-754 result including its sign; normalizing -0 is Format's job.
func identical(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b)
}

// countSignificantDigits ignores sign, point, exponent, and leading zeros, so the 12-digit
// ceiling can be asserted as a property rather than case by case.
func countSignificantDigits(formatted string) int {
	mantissa := formatted
	if i := strings.IndexAny(mantissa, "eE"); i >= 0 {
		mantissa = mantissa[:i]
	}
	mantissa = strings.TrimPrefix(mantissa, "-")
	mantissa = strings.ReplaceAll(mantissa, ".", "")

	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return 0
	}
	return len(digits)
}
