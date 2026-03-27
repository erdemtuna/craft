package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestMultiSelectSpecificItems(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("1,3\n")

	items := []string{"alpha", "beta", "gamma", "delta"}
	got, err := MultiSelect("Pick skills:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("expected [0, 2], got %v", got)
	}

	// Verify output contains numbered list
	output := out.String()
	if !strings.Contains(output, "[1] alpha") {
		t.Errorf("expected numbered list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "[4] delta") {
		t.Errorf("expected [4] delta in output, got:\n%s", output)
	}
}

func TestMultiSelectAll(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("a\n")

	items := []string{"alpha", "beta", "gamma"}
	got, err := MultiSelect("Pick:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (all), got %v", got)
	}
}

func TestMultiSelectEmpty(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("\n")

	items := []string{"alpha", "beta"}
	got, err := MultiSelect("Pick:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (all), got %v", got)
	}
}

func TestMultiSelectSingle(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("1\n")

	items := []string{"alpha", "beta", "gamma"}
	got, err := MultiSelect("Pick:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("expected [0], got %v", got)
	}
}

func TestMultiSelectOutOfRange(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("5\n")

	items := []string{"alpha", "beta"}
	_, err := MultiSelect("Pick:", items, &out, r)
	if err == nil {
		t.Fatal("expected error for out-of-range number")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out-of-range error, got: %v", err)
	}
}

func TestMultiSelectInvalidInput(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("abc\n")

	items := []string{"alpha", "beta"}
	_, err := MultiSelect("Pick:", items, &out, r)
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if !strings.Contains(err.Error(), "invalid number") {
		t.Errorf("expected invalid number error, got: %v", err)
	}
}

func TestMultiSelectAllExplicit(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("all\n")

	items := []string{"alpha", "beta"}
	got, err := MultiSelect("Pick:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (all), got %v", got)
	}
}

func TestMultiSelectAllIndicesReturnsNil(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("1,2,3\n")

	items := []string{"alpha", "beta", "gamma"}
	got, err := MultiSelect("Pick:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil when all items selected, got %v", got)
	}
}

func TestMultiSelectDuplicateNumbers(t *testing.T) {
	var out bytes.Buffer
	r := strings.NewReader("1,1,2\n")

	items := []string{"alpha", "beta", "gamma"}
	got, err := MultiSelect("Pick:", items, &out, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("expected [0, 1] (deduped), got %v", got)
	}
}
