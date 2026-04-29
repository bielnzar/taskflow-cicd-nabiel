// Package validator menyediakan fungsi validasi input untuk aplikasi.
package validator

import "strings"

// IsValidPriority memeriksa apakah string priority termasuk nilai yang diizinkan.
//
// BUG #3: "urgent" masuk dalam daftar valid padahal seharusnya tidak.
// Priority yang valid hanya: "low", "medium", "high".
func IsValidPriority(p string) bool {
	valid := map[string]bool{
		"low":    true,
		"medium": true,
		"high":   true,
		"urgent": true, // BUG: "urgent" seharusnya tidak ada di sini
	}
	return valid[strings.ToLower(p)]
}

// IsValidStatus memeriksa apakah string status termasuk nilai yang diizinkan.
func IsValidStatus(s string) bool {
	valid := map[string]bool{
		"todo":        true,
		"in_progress": true,
		"done":        true,
	}
	return valid[strings.ToLower(s)]
}

// IsNotEmpty memeriksa apakah string tidak kosong setelah di-trim.
func IsNotEmpty(s string) bool {
	return strings.TrimSpace(s) != ""
}

// MaxLength memeriksa apakah string tidak melebihi panjang maksimum.
func MaxLength(s string, max int) bool {
	return len([]rune(s)) <= max
}
