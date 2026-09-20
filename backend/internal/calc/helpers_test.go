package calc

import (
	"math"
	"strconv"
	"strings"
)

// num parses a float64 at runtime.
//
// Every operand and expected value in this package is written as a string and passed
// through here. Go evaluates untyped constant expressions in arbitrary precision, so a
// table written as `0.1 + 0.2` would be folded to exactly 0.3 at compile time and the test
// would pass without ever exercising float64 (CLAUDE.md "Testing"; ADR-0015).
func num(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic("test table holds a value that is not a float64 literal: " + s)
	}
	return v
}

// nums parses a slice of operands at runtime.
func nums(ss []string) []float64 {
	out := make([]float64, len(ss))
	for i, s := range ss {
		out[i] = num(s)
	}
	return out
}

// identical compares bit patterns rather than values, so that negative zero is
// distinguishable from positive zero. The arithmetic layer is expected to return the raw
// IEEE-754 result including its sign; normalization of -0 belongs to Format alone
// (ADR-0015 step 2).
func identical(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b)
}

// countSignificantDigits counts the digits a formatted result actually carries, ignoring sign,
// decimal point, exponent, and leading zeros. Used to assert the 12-digit ceiling of
// ADR-0003 clause 2 as a property rather than case by case.
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
