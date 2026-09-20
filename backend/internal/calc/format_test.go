package calc

import (
	"math"
	"strconv"
	"testing"
)

// The rounding policy of ADR-0015, case by case. Every expectation was produced by
// executing the algorithm against runtime binary64 values, never written by hand.
var formatCases = []struct {
	name  string
	value string
	want  string
}{
	{"representation error in a sum", "0.30000000000000004", "0.3"},
	{"representation error in a difference", "0.19999999999999998", "0.2"},
	{"representation error in a product", "1.2100000000000002", "1.21"},
	{"negative representation error", "-0.30000000000000004", "-0.3"},
	{"recurring decimal truncated to twelve digits", "0.3333333333333333", "0.333333333333"},
	{"irrational root truncated to twelve digits", "1.4142135623730951", "1.41421356237"},

	{"exact halves are untouched", "2.5", "2.5"},
	{"integers carry no decimal point", "1024", "1024"},
	{"trailing zeros are stripped", "2.50", "2.5"},
	{"a whole number written with a fraction loses it", "30.000", "30"},

	{"positive zero", "0", "0"},
	{"negative zero is normalized", "-0", "0"},

	{"large product crosses into scientific notation", "1.2193263111263526e+17", "1.21932631113e+17"},
	{"largest representable value", "1.7976931348623157e308", "1.79769313486e+308"},
	{"smallest subnormal", "5e-324", "5e-324"},
	{"subnormal", "1e-320", "1e-320"},
}

func TestFormatAppliesTheRoundingPolicy(t *testing.T) {
	for _, tt := range formatCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(num(tt.value)); got != tt.want {
				t.Errorf("Format(%s) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// Decimal inside [1e-9, 1e12), scientific outside. The band edges are where a single 'g'
// call would diverge from the policy (ADR-0015).
func TestFormatSelectsNotationByMagnitudeBand(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"just inside the upper edge", "999999999999", "999999999999"},
		{"on the upper edge", "1e12", "1e+12"},
		{"above the upper edge", "1234567890123", "1.23456789012e+12"},
		{"negative on the upper edge", "-1e12", "-1e+12"},
		{"well inside the decimal band", "1e11", "100000000000"},

		{"on the lower edge", "1e-9", "0.000000001"},
		{"just inside the lower edge", "9.99999999999e-10", "9.99999999999e-10"},
		{"below the lower edge", "1e-10", "1e-10"},
		{"negative below the lower edge", "-1e-10", "-1e-10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(num(tt.value)); got != tt.want {
				t.Errorf("Format(%s) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// Notation follows the rounded value, so one that rounds across a band edge is rendered in
// the band it lands in.
func TestFormatSelectsNotationAfterRounding(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"rounds up over the upper edge into scientific", "999999999999.9", "1e+12"},
		{"rounds up into the decimal band", "9.999999999999e-10", "0.000000001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(num(tt.value)); got != tt.want {
				t.Errorf("Format(%s) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// Round-half-to-even, not half-away-from-zero. Both values are exactly representable and
// carry thirteen significant digits ending in 5, so the tie is genuine.
func TestFormatRoundsHalfToEven(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"tie keeps an even last digit", "1073741824.125", "1073741824.12"},
		{"tie rounds an odd last digit up", "1073741824.375", "1073741824.38"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(num(tt.value)); got != tt.want {
				t.Errorf("Format(%s) = %q, want %q (half-away-from-zero would give a different answer)",
					tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatNeverExceedsTwelveSignificantDigits(t *testing.T) {
	values := []string{
		"0.3333333333333333", "1.4142135623730951", "1.2193263111263526e+17",
		"1.7976931348623157e308", "0.30000000000000004", "999999999999.9",
		"1234567890123.4567", "0.123456789012345", "-98765.4321098765432",
	}

	for _, v := range values {
		got := Format(num(v))
		if n := countSignificantDigits(got); n > 12 {
			t.Errorf("Format(%s) = %q, which carries %d significant digits; the policy caps them at 12", v, got, n)
		}
	}
}

// Re-formatting a formatted result must be a no-op, or a chained calculation would drift
// every time a result is re-entered as an operand (ADR-0009).
func TestFormatIsStableUnderReparsing(t *testing.T) {
	for _, tt := range formatCases {
		t.Run(tt.name, func(t *testing.T) {
			once := Format(num(tt.value))

			reparsed, err := strconv.ParseFloat(once, 64)
			if err != nil {
				t.Fatalf("Format(%s) = %q, which is not a parseable number: %v", tt.value, once, err)
			}
			if math.IsNaN(reparsed) || math.IsInf(reparsed, 0) {
				t.Fatalf("Format(%s) = %q, which parses to a non-finite value", tt.value, once)
			}

			if twice := Format(reparsed); twice != once {
				t.Errorf("Format is not idempotent: %q became %q", once, twice)
			}
		})
	}
}

// The pairs published in docs/API_EXAMPLES.md: arithmetic and rounding policy composed.
func TestApplyThenFormatMatchesPublishedExamples(t *testing.T) {
	tests := []struct {
		op       string
		operands []string
		want     string
	}{
		{"add", []string{"0.1", "0.2"}, "0.3"},
		{"subtract", []string{"0.3", "0.1"}, "0.2"},
		{"multiply", []string{"1.1", "1.1"}, "1.21"},
		{"divide", []string{"10", "4"}, "2.5"},
		{"divide", []string{"1", "3"}, "0.333333333333"},
		{"power", []string{"2", "10"}, "1024"},
		{"sqrt", []string{"2"}, "1.41421356237"},
		{"percent", []string{"15", "200"}, "30"},

		{"multiply", []string{"123456789", "987654321"}, "1.21932631113e+17"},
		{"divide", []string{"1", "1000000000"}, "0.000000001"},
		{"divide", []string{"1", "10000000000"}, "1e-10"},
		{"multiply", []string{"0", "-5"}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.op+"/"+tt.want, func(t *testing.T) {
			got, err := Apply(tt.op, nums(tt.operands))
			if err != nil {
				t.Fatalf("Apply(%q, %v) returned error %v", tt.op, tt.operands, err)
			}
			if formatted := Format(got); formatted != tt.want {
				t.Errorf("Format(Apply(%q, %v)) = %q, want %q (raw was %v)",
					tt.op, tt.operands, formatted, tt.want, got)
			}
		})
	}
}
