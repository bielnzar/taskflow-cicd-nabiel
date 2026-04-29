package validator_test

import (
	"testing"

	"github.com/taskflow/api/internal/validator"
)

// ── [BUG] IsValidPriority ────────────────────────────────────────────────────
// BUG #3: "urgent" seharusnya TIDAK valid.

func TestIsValidPriority(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
		isBug bool // tandai test yang akan GAGAL karena bug
	}{
		{name: "low valid", input: "low", want: true},
		{name: "medium valid", input: "medium", want: true},
		{name: "high valid", input: "high", want: true},
		// [BUG] test berikut AKAN GAGAL karena bug #3
		{name: "[BUG] urgent harus invalid", input: "urgent", want: false, isBug: true},
		{name: "critical tidak valid", input: "critical", want: false},
		{name: "string kosong tidak valid", input: "", want: false},
		{name: "case-insensitive HIGH", input: "HIGH", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validator.IsValidPriority(tc.input)
			if got != tc.want {
				if tc.isBug {
					t.Errorf("BUG TERDETEKSI — IsValidPriority(%q) = %v, want %v\n"+
						"  → Perbaiki: hapus 'urgent' dari daftar valid di validator.go", tc.input, got, tc.want)
				} else {
					t.Errorf("IsValidPriority(%q) = %v, want %v", tc.input, got, tc.want)
				}
			}
		})
	}
}

// ── IsValidStatus ─────────────────────────────────────────────────────────────

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"todo", true},
		{"in_progress", true},
		{"done", true},
		{"pending", false},
		{"canceled", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := validator.IsValidStatus(tc.input); got != tc.want {
				t.Errorf("IsValidStatus(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── IsNotEmpty ────────────────────────────────────────────────────────────────

func TestIsNotEmpty(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{"  spasi  ", true},
		{"", false},
		{"   ", false},
		{"\t\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := validator.IsNotEmpty(tc.input); got != tc.want {
				t.Errorf("IsNotEmpty(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── MaxLength ─────────────────────────────────────────────────────────────────

func TestMaxLength(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  bool
	}{
		{"hello", 10, true},
		{"hello", 5, true},
		{"hello", 4, false},
		{"", 0, true},
		// Unicode: 3 karakter emoji tapi bisa lebih dari 3 byte
		{"🚀🎯✅", 3, true},
		{"🚀🎯✅", 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := validator.MaxLength(tc.input, tc.max); got != tc.want {
				t.Errorf("MaxLength(%q, %d) = %v, want %v", tc.input, tc.max, got, tc.want)
			}
		})
	}
}
