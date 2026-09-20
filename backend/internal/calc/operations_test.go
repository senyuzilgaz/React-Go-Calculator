package calc

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

// Apply returns the raw IEEE-754 result. Rounding and negative-zero normalization are
// Format's job and no one else's (ADR-0003 clause 1, ADR-0015), so these expectations are
// the unrounded binary64 values — including -0 where the sign survives.
func TestApplyReturnsRawBinary64Results(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		operands []string
		want     string
	}{
		{"add absorbs nothing on its own", "add", []string{"0.1", "0.2"}, "0.30000000000000004"},
		{"add whole numbers", "add", []string{"2", "3"}, "5"},
		{"add negative", "add", []string{"-2.5", "1.25"}, "-1.25"},
		{"add to positive zero", "add", []string{"-1.5", "1.5"}, "0"},

		{"subtract representation error", "subtract", []string{"0.3", "0.1"}, "0.19999999999999998"},
		{"subtract to zero", "subtract", []string{"2", "2"}, "0"},
		{"subtract past zero", "subtract", []string{"1", "2"}, "-1"},

		{"multiply representation error", "multiply", []string{"1.1", "1.1"}, "1.2100000000000002"},
		{"multiply whole numbers", "multiply", []string{"6", "7"}, "42"},
		{"multiply yields negative zero", "multiply", []string{"0", "-5"}, "-0"},
		{"multiply two negatives cancels the sign", "multiply", []string{"-0", "-5"}, "0"},
		{"multiply underflows to zero rather than failing", "multiply", []string{"1e-200", "1e-200"}, "0"},

		{"divide exactly", "divide", []string{"10", "4"}, "2.5"},
		{"divide recurring", "divide", []string{"1", "3"}, "0.3333333333333333"},
		{"divide recurring negative", "divide", []string{"-1", "3"}, "-0.3333333333333333"},
		{"divide underflows to zero rather than failing", "divide", []string{"1e-200", "1e200"}, "0"},

		{"power integral", "power", []string{"2", "10"}, "1024"},
		{"power zero exponent", "power", []string{"7", "0"}, "1"},
		{"power negative exponent", "power", []string{"2", "-1"}, "0.5"},
		{"power negative base integral exponent", "power", []string{"-2", "3"}, "-8"},
		{"power fractional exponent is a root", "power", []string{"2", "0.5"}, "1.4142135623730951"},

		{"sqrt exact", "sqrt", []string{"9"}, "3"},
		{"sqrt irrational", "sqrt", []string{"2"}, "1.4142135623730951"},
		{"sqrt zero", "sqrt", []string{"0"}, "0"},

		{"percent of a value", "percent", []string{"15", "200"}, "30"},
		{"percent fractional", "percent", []string{"7.5", "33"}, "2.475"},
		{"percent small", "percent", []string{"0.1", "0.2"}, "0.00020000000000000004"},
		{"percent of zero", "percent", []string{"50", "0"}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.op, nums(tt.operands))
			if err != nil {
				t.Fatalf("Apply(%q, %v) returned error %v, want %s", tt.op, tt.operands, err, tt.want)
			}
			if want := num(tt.want); !identical(got, want) {
				t.Errorf("Apply(%q, %v) = %v (bits %#x), want %v (bits %#x)",
					tt.op, tt.operands, got, math.Float64bits(got), want, math.Float64bits(want))
			}
		})
	}
}

// 0^0 is 1 by the IEEE-754 pow convention, which is what math.Pow implements. It is a
// deliberate answer rather than an undefined form, so it is not an error.
func TestPowerOfZeroToTheZeroIsOne(t *testing.T) {
	got, err := Apply("power", nums([]string{"0", "0"}))
	if err != nil {
		t.Fatalf("power(0, 0) returned error %v, want 1", err)
	}
	if want := num("1"); !identical(got, want) {
		t.Errorf("power(0, 0) = %v, want 1", got)
	}
}

// -0 is not a negative number: the radicand guard is `x < 0`, not a sign-bit test.
// math.Sqrt(-0) is -0, which Format later normalizes to "0".
func TestSqrtOfNegativeZeroSucceeds(t *testing.T) {
	got, err := Apply("sqrt", nums([]string{"-0"}))
	if err != nil {
		t.Fatalf("sqrt(-0) returned error %v, want -0", err)
	}
	if want := num("-0"); !identical(got, want) {
		t.Errorf("sqrt(-0) = %v (bits %#x), want -0 (bits %#x)",
			got, math.Float64bits(got), math.Float64bits(want))
	}
	if Format(got) != "0" {
		t.Errorf("Format(sqrt(-0)) = %q, want %q", Format(got), "0")
	}
}

// ADR-0010: percent is a plain binary operation, a x b / 100 — "a percent of b". The
// contextual calculator behaviour where 200 + 10 % yields 220 is explicitly out of scope.
func TestPercentIsANonContextualBinaryOperation(t *testing.T) {
	got, err := Apply("percent", nums([]string{"200", "10"}))
	if err != nil {
		t.Fatalf("percent(200, 10) returned error %v", err)
	}
	if want := num("20"); !identical(got, want) {
		t.Errorf("percent(200, 10) = %v, want 20 (a percent of b, not a contextual percentage)", got)
	}

	// a x b / 100 is symmetric in its operands, which the ordering of the catalog's
	// percentage/value roles does not change.
	swapped, err := Apply("percent", nums([]string{"10", "200"}))
	if err != nil {
		t.Fatalf("percent(10, 200) returned error %v", err)
	}
	if !identical(got, swapped) {
		t.Errorf("percent(200, 10) = %v but percent(10, 200) = %v", got, swapped)
	}
}

// The formula is fixed as a x b / 100 (ADR-0010), so the product is formed first and can
// overflow even when the true result is representable: 1e300 percent of 1e10 is 1e308.
// This pins the order of evaluation, not just the value.
func TestPercentFormsTheProductBeforeDividing(t *testing.T) {
	_, err := Apply("percent", nums([]string{"1e300", "1e10"}))
	if !errors.Is(err, ErrResultOverflow) {
		t.Fatalf("percent(1e300, 1e10) error = %v, want ErrResultOverflow; the intermediate "+
			"product 1e310 overflows under the a x b / 100 formula of ADR-0010", err)
	}
}

func TestApplyDomainErrors(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		operands []string
		want     error
	}{
		{"divide by zero", "divide", []string{"10", "0"}, ErrDivisionByZero},
		{"zero divided by zero", "divide", []string{"0", "0"}, ErrDivisionByZero},
		{"negative dividend by zero", "divide", []string{"-5", "0"}, ErrDivisionByZero},
		{"divide by negative zero", "divide", []string{"10", "-0"}, ErrDivisionByZero},

		{"sqrt of a negative", "sqrt", []string{"-4"}, ErrNegativeSqrt},
		{"sqrt of a tiny negative", "sqrt", []string{"-1e-300"}, ErrNegativeSqrt},
		{"sqrt of the most negative value", "sqrt", []string{"-1.7976931348623157e308"}, ErrNegativeSqrt},

		{"addition overflows", "add", []string{"1e308", "1e308"}, ErrResultOverflow},
		{"subtraction overflows negative", "subtract", []string{"-1e308", "1e308"}, ErrResultOverflow},
		{"multiplication overflows", "multiply", []string{"1e200", "1e200"}, ErrResultOverflow},
		{"multiplication overflows negative", "multiply", []string{"-1e200", "1e200"}, ErrResultOverflow},
		{"division overflows", "divide", []string{"1e308", "1e-10"}, ErrResultOverflow},
		{"power overflows", "power", []string{"1e308", "2"}, ErrResultOverflow},
		{"power overflows on a large exponent", "power", []string{"2", "1024"}, ErrResultOverflow},

		// Non-finite results have exactly one code regardless of how they arose: a pole
		// and an undefined form are both RESULT_OVERFLOW (ADR-0004).
		{"zero to a negative power is a pole", "power", []string{"0", "-1"}, ErrResultOverflow},
		{"negative base to a fractional exponent is NaN", "power", []string{"-1", "0.5"}, ErrResultOverflow},
		{"negative cube root is NaN", "power", []string{"-8", "0.3333333333333333"}, ErrResultOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(tt.op, nums(tt.operands))
			if !errors.Is(err, tt.want) {
				t.Fatalf("Apply(%q, %v) error = %v, want %v (result was %v)",
					tt.op, tt.operands, err, tt.want, got)
			}
		})
	}
}

// Non-finite values never reach the wire (CLAUDE.md invariant). JSON cannot carry NaN or
// +/-Inf, but the domain layer is the authority and rejects them regardless of how they
// arrived (ADR-0005), on every operation and at every position.
func TestApplyRejectsNonFiniteOperands(t *testing.T) {
	nonFinite := map[string]float64{
		"NaN":       math.NaN(),
		"+Inf":      math.Inf(1),
		"-Inf":      math.Inf(-1),
		"0/0":       num("0") / num("0"),
		"overflown": num("1e308") * num("10"),
	}

	for _, op := range Catalog() {
		for label, bad := range nonFinite {
			for pos := 0; pos < op.Arity; pos++ {
				t.Run(op.ID+"/"+label+"/position"+strconv.Itoa(pos), func(t *testing.T) {
					operands := validOperandsFor(op)
					operands[pos] = bad

					got, err := Apply(op.ID, operands)
					if !errors.Is(err, ErrInvalidOperand) {
						t.Fatalf("Apply(%q, %v) error = %v, want ErrInvalidOperand (result was %v)",
							op.ID, operands, err, got)
					}
				})
			}
		}
	}
}

// Arity is enforced against the registry, not hard-coded per operation (ADR-0002).
func TestApplyRejectsWrongOperandCount(t *testing.T) {
	for _, op := range Catalog() {
		for count := 0; count <= 3; count++ {
			if count == op.Arity {
				continue
			}
			t.Run(op.ID+"/"+strconv.Itoa(count)+"operands", func(t *testing.T) {
				operands := make([]float64, count)
				for i := range operands {
					operands[i] = num("4")
				}

				if _, err := Apply(op.ID, operands); !errors.Is(err, ErrWrongOperandCount) {
					t.Fatalf("Apply(%q, %d operands) error = %v, want ErrWrongOperandCount (arity is %d)",
						op.ID, count, err, op.Arity)
				}
			})
		}

		t.Run(op.ID+"/nil", func(t *testing.T) {
			if _, err := Apply(op.ID, nil); !errors.Is(err, ErrWrongOperandCount) {
				t.Fatalf("Apply(%q, nil) error = %v, want ErrWrongOperandCount", op.ID, err)
			}
		})
	}
}

func TestApplyRejectsUnknownOperation(t *testing.T) {
	for _, id := range []string{"modulo", "ADD", "", "sqr", "factorial"} {
		if _, err := Apply(id, nums([]string{"1", "2"})); !errors.Is(err, ErrUnknownOperation) {
			t.Errorf("Apply(%q, ...) error = %v, want ErrUnknownOperation", id, err)
		}
	}
}

// When several failures apply at once, the classification has to be stable, because each
// maps to a different status code (ADR-0004): existence first, then shape, then values.
func TestApplyErrorPrecedence(t *testing.T) {
	t.Run("unknown operation outranks a bad request body", func(t *testing.T) {
		_, err := Apply("modulo", []float64{math.NaN(), math.Inf(1), math.NaN()})
		if !errors.Is(err, ErrUnknownOperation) {
			t.Fatalf("error = %v, want ErrUnknownOperation (404 before 400)", err)
		}
	})

	t.Run("operand count outranks operand validity", func(t *testing.T) {
		_, err := Apply("sqrt", []float64{math.NaN(), math.NaN()})
		if !errors.Is(err, ErrWrongOperandCount) {
			t.Fatalf("error = %v, want ErrWrongOperandCount", err)
		}
	})

	t.Run("operand validity outranks the computation", func(t *testing.T) {
		// A malformed request is a 400 and must not be reported as the 422 the
		// computation would have produced.
		_, err := Apply("divide", []float64{math.NaN(), num("0")})
		if !errors.Is(err, ErrInvalidOperand) {
			t.Fatalf("error = %v, want ErrInvalidOperand, not ErrDivisionByZero", err)
		}
	})
}

// The handler echoes the operands it received back to the client, so the domain layer must
// treat the slice it is given as read-only.
func TestApplyDoesNotMutateOperands(t *testing.T) {
	for _, op := range Catalog() {
		t.Run(op.ID, func(t *testing.T) {
			operands := validOperandsFor(op)
			before := append([]float64(nil), operands...)

			if _, err := Apply(op.ID, operands); err != nil {
				t.Fatalf("Apply(%q, %v) returned error %v", op.ID, operands, err)
			}

			for i := range before {
				if !identical(operands[i], before[i]) {
					t.Errorf("operand %d changed from %v to %v", i, before[i], operands[i])
				}
			}
		})
	}
}

// Apply is the guarded entry point; the registry entry is the bare arithmetic behind it.
// Both must agree on the value for inputs that pass the guards.
func TestRegistryApplyAgreesWithPackageApply(t *testing.T) {
	for _, op := range Catalog() {
		t.Run(op.ID, func(t *testing.T) {
			operands := validOperandsFor(op)

			viaPackage, err := Apply(op.ID, operands)
			if err != nil {
				t.Fatalf("Apply(%q, %v) returned error %v", op.ID, operands, err)
			}
			viaRegistry, err := op.Apply(operands)
			if err != nil {
				t.Fatalf("%s.Apply(%v) returned error %v", op.ID, operands, err)
			}

			if !identical(viaPackage, viaRegistry) {
				t.Errorf("Apply(%q) = %v but registry entry returned %v", op.ID, viaPackage, viaRegistry)
			}
		})
	}
}

// Error text never reaches a client (ADR-0004), but it does reach the log, where it is
// useless without naming what failed.
func TestErrorsCarryDiagnosticContext(t *testing.T) {
	t.Run("unknown operation names the identifier", func(t *testing.T) {
		_, err := Apply("modulo", nums([]string{"10", "3"}))
		if err == nil {
			t.Fatal("Apply(\"modulo\", ...) returned no error")
		}
		if !strings.Contains(err.Error(), "modulo") {
			t.Errorf("error %q does not name the unknown operation", err)
		}
	})

	t.Run("operand count names the operation and both counts", func(t *testing.T) {
		_, err := Apply("sqrt", nums([]string{"9", "16"}))
		if err == nil {
			t.Fatal("Apply(\"sqrt\", two operands) returned no error")
		}
		for _, want := range []string{"sqrt", "1", "2"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not mention %q (want operation, required arity, received count)", err, want)
			}
		}
	})

	// Positions are 1-based, matching the published message "Operand at position 1 is not
	// a finite number" for the first operand (api/openapi.yaml, docs/API_EXAMPLES.md).
	t.Run("invalid operand names its position", func(t *testing.T) {
		_, err := Apply("add", []float64{num("1"), math.NaN()})
		if err == nil {
			t.Fatal("Apply(\"add\", [1, NaN]) returned no error")
		}
		if !strings.Contains(err.Error(), "position 2") {
			t.Errorf("error %q does not identify which operand was rejected", err)
		}
	})
}

// validOperandsFor builds operands that every published operation accepts: 4, or 4 and 2.
func validOperandsFor(op Operation) []float64 {
	operands := make([]float64, 0, op.Arity)
	for _, s := range []string{"4", "2"}[:op.Arity] {
		operands = append(operands, num(s))
	}
	return operands
}
