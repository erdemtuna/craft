package ui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// MultiSelect presents a numbered list of items and lets the user select
// which to include by entering numbers. Returns indices of selected items.
// If all are selected, returns nil (meaning "all").
func MultiSelect(prompt string, items []string, w io.Writer, r io.Reader) ([]int, error) {
	_, _ = fmt.Fprintln(w, prompt)
	for i, item := range items {
		_, _ = fmt.Fprintf(w, "  [%d] %s\n", i+1, item)
	}
	_, _ = fmt.Fprint(w, "Enter numbers to include (e.g. 1,3,5), 'a' for all, or Enter for all: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading selection: %w", err)
		}
		// EOF — treat as "all"
		return nil, nil
	}

	input := strings.TrimSpace(scanner.Text())

	// Empty or "all" → select everything
	if input == "" || strings.EqualFold(input, "a") || strings.EqualFold(input, "all") {
		return nil, nil
	}

	// Parse comma-separated numbers
	parts := strings.Split(input, ",")
	seen := make(map[int]bool)
	var indices []int
	for _, part := range parts {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		num, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q — enter comma-separated numbers (e.g. 1,3,5)", s)
		}
		if num < 1 || num > len(items) {
			return nil, fmt.Errorf("number %d out of range (1–%d)", num, len(items))
		}
		idx := num - 1
		if !seen[idx] {
			seen[idx] = true
			indices = append(indices, idx)
		}
	}

	if len(indices) == 0 {
		return nil, nil
	}

	// If all selected, return nil
	if len(indices) == len(items) {
		return nil, nil
	}

	return indices, nil
}
