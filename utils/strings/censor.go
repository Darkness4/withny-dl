package strings

import (
	"fmt"
	"strings"
)

// Censor masks a string for debugging, showing a few characters at the start and end.
// It uses a mask character (e.g., '*') and includes the total length.
func Censor(s string, visibleChars int, mask string) string {
	sLen := len(s)
	maskLen := len(mask)

	// 1. Handle short/empty strings
	if sLen <= 2*visibleChars+maskLen {
		// If the string is already short enough, return it unmasked with its length.
		// For very short strings, masking is often less useful.
		return fmt.Sprintf("%s (len: %d)", s, sLen)
	}

	// 2. Calculate segments
	// We want 'visibleChars' at the start and 'visibleChars' at the end.
	start := s[:visibleChars]
	end := s[sLen-visibleChars:]

	// 3. Construct the masked part
	// The number of characters to be masked is the original length minus the visible parts.
	// We'll just use a fixed number of mask characters (e.g., "*****") for clarity,
	// rather than a length-accurate mask.
	maskedPart := strings.Repeat(mask, 5) // Use 5 mask characters for a clear separator

	// 4. Combine and append length
	// Result: "start" + "*****" + "end" + " (len: N)"
	return fmt.Sprintf("%s%s%s (len: %d)", start, maskedPart, end, sLen)
}
