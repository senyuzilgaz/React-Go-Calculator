package calc

import (
	"errors"
	"reflect"
	"testing"
)

// publishedCatalog mirrors api/openapi.yaml and docs/API_EXAMPLES.md, order included. The
// catalog endpoint is generated from the registry, so divergence here breaks the contract.
var publishedCatalog = []Operation{
	{
		ID:     "add",
		Name:   "Addition",
		Symbol: "+",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "augend", Description: "The value added to."},
			{Name: "addend", Description: "The value added."},
		},
	},
	{
		ID:     "subtract",
		Name:   "Subtraction",
		Symbol: "−",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "minuend", Description: "The value subtracted from."},
			{Name: "subtrahend", Description: "The value subtracted."},
		},
	},
	{
		ID:     "multiply",
		Name:   "Multiplication",
		Symbol: "×",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "multiplicand", Description: "The value multiplied."},
			{Name: "multiplier", Description: "The number of times it is taken."},
		},
	},
	{
		ID:     "divide",
		Name:   "Division",
		Symbol: "÷",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "dividend", Description: "The value divided."},
			{Name: "divisor", Description: "The value divided by. Must not be zero."},
		},
	},
	{
		ID:     "power",
		Name:   "Exponentiation",
		Symbol: "^",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "base", Description: "The value raised to a power."},
			{Name: "exponent", Description: "The power the base is raised to."},
		},
	},
	{
		ID:     "sqrt",
		Name:   "Square root",
		Symbol: "√",
		Arity:  1,
		Parameters: []Parameter{
			{Name: "radicand", Description: "The value whose square root is taken. Must not be negative."},
		},
	},
	{
		ID:     "percent",
		Name:   "Percentage",
		Symbol: "%",
		Arity:  2,
		Parameters: []Parameter{
			{Name: "percentage", Description: "The percentage to take."},
			{Name: "value", Description: "The value the percentage is taken of."},
		},
	},
}

func TestCatalogMatchesPublishedContract(t *testing.T) {
	got := Catalog()

	if len(got) != len(publishedCatalog) {
		t.Fatalf("Catalog() returned %d operations, contract publishes %d: got %v",
			len(got), len(publishedCatalog), operationIDs(got))
	}

	for i, want := range publishedCatalog {
		op := got[i]
		if op.ID != want.ID {
			t.Errorf("Catalog()[%d].ID = %q, want %q (catalog order is part of the contract)", i, op.ID, want.ID)
			continue
		}
		if op.Name != want.Name {
			t.Errorf("%s: Name = %q, want %q", want.ID, op.Name, want.Name)
		}
		if op.Symbol != want.Symbol {
			t.Errorf("%s: Symbol = %q, want %q", want.ID, op.Symbol, want.Symbol)
		}
		if op.Arity != want.Arity {
			t.Errorf("%s: Arity = %d, want %d", want.ID, op.Arity, want.Arity)
		}
		if !reflect.DeepEqual(op.Parameters, want.Parameters) {
			t.Errorf("%s: Parameters = %+v, want %+v", want.ID, op.Parameters, want.Parameters)
		}
	}
}

func TestCatalogParameterCountEqualsArity(t *testing.T) {
	for _, op := range Catalog() {
		if len(op.Parameters) != op.Arity {
			t.Errorf("%s: %d parameters for arity %d; the catalog promises they are equal",
				op.ID, len(op.Parameters), op.Arity)
		}
	}
}

// Catalog-driven UI labels are unambiguous only if roles are unique per operation.
func TestCatalogParameterNamesAreUniqueWithinOperation(t *testing.T) {
	for _, op := range Catalog() {
		seen := make(map[string]bool, len(op.Parameters))
		for _, p := range op.Parameters {
			if p.Name == "" {
				t.Errorf("%s: parameter with empty name", op.ID)
			}
			if p.Description == "" {
				t.Errorf("%s: parameter %q has no description", op.ID, p.Name)
			}
			if seen[p.Name] {
				t.Errorf("%s: duplicate parameter name %q", op.ID, p.Name)
			}
			seen[p.Name] = true
		}
	}
}

func TestCatalogIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, op := range Catalog() {
		if seen[op.ID] {
			t.Errorf("duplicate operation id %q in catalog", op.ID)
		}
		seen[op.ID] = true
	}
}

func TestCatalogEveryOperationIsApplicable(t *testing.T) {
	for _, op := range Catalog() {
		if op.Apply == nil {
			t.Errorf("%s: Apply is nil; the registry drives execution as well as the catalog", op.ID)
		}
	}
}

// The registry is process-wide state. A caller editing what it was handed must not be able
// to corrupt it for the next caller — including through the Parameters slice, which a
// shallow copy of the struct would leave aliasing the registry (ADR-0022).
func TestCatalogReturnsIndependentOperations(t *testing.T) {
	first := Catalog()
	if len(first) == 0 {
		t.Fatal("Catalog() is empty")
	}

	original := first[0].Parameters[0].Name
	first[0].Parameters[0].Name = "tampered"
	first[0] = Operation{ID: "tampered"}

	second := Catalog()
	if second[0].ID == "tampered" {
		t.Error("mutating the slice returned by Catalog() changed the registry")
	}
	if got := second[0].Parameters[0].Name; got != original {
		t.Errorf("Parameters[0].Name = %q, want %q; Catalog() hands back registry state", got, original)
	}
}

func TestLookupReturnsIndependentOperations(t *testing.T) {
	first, ok := Lookup("add")
	if !ok {
		t.Fatal("add is not registered")
	}

	original := first.Parameters[0].Name
	first.Parameters[0].Name = "tampered"

	second, _ := Lookup("add")
	if got := second.Parameters[0].Name; got != original {
		t.Errorf("Parameters[0].Name = %q, want %q; Lookup hands back registry state", got, original)
	}
}

func TestLookupReturnsRegisteredOperations(t *testing.T) {
	for _, want := range publishedCatalog {
		op, ok := Lookup(want.ID)
		if !ok {
			t.Errorf("Lookup(%q) reported not found, but it is published in the catalog", want.ID)
			continue
		}
		if op.ID != want.ID {
			t.Errorf("Lookup(%q).ID = %q", want.ID, op.ID)
		}
		if op.Arity != want.Arity {
			t.Errorf("Lookup(%q).Arity = %d, want %d", want.ID, op.Arity, want.Arity)
		}
	}
}

func TestLookupRejectsUnknownIdentifiers(t *testing.T) {
	// Identifiers are exact: an unrecognised segment is a 404, never a near match.
	for _, id := range []string{"modulo", "ADD", "Add", "sqr", "sqrtt", " add", "add ", "", "+", "divide/"} {
		if _, ok := Lookup(id); ok {
			t.Errorf("Lookup(%q) resolved to an operation; it is not in the published catalog", id)
		}
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	sentinels := map[string]error{
		"ErrUnknownOperation":  ErrUnknownOperation,
		"ErrWrongOperandCount": ErrWrongOperandCount,
		"ErrInvalidOperand":    ErrInvalidOperand,
		"ErrDivisionByZero":    ErrDivisionByZero,
		"ErrNegativeSqrt":      ErrNegativeSqrt,
		"ErrResultOverflow":    ErrResultOverflow,
	}

	// Each sentinel maps to a distinct wire code; conflating two would collapse a 400 into
	// a 422 or hide one failure mode behind another.
	for nameA, a := range sentinels {
		if a == nil {
			t.Errorf("%s is nil", nameA)
			continue
		}
		if a.Error() == "" {
			t.Errorf("%s has an empty message", nameA)
		}
		for nameB, b := range sentinels {
			if nameA == nameB {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("errors.Is(%s, %s) is true; the sentinels must classify distinctly", nameA, nameB)
			}
		}
	}
}

func operationIDs(ops []Operation) []string {
	ids := make([]string, len(ops))
	for i, op := range ops {
		ids[i] = op.ID
	}
	return ids
}
