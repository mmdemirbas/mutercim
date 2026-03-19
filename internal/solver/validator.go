package solver

import (
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// Validate checks structural consistency of a read page.
// Validates hadith number sequencing and flags anomalies.
func Validate(page *model.ReadPage) *model.Validation {
	v := &model.Validation{
		Status:                    "ok",
		HadithNumberSequenceValid: true,
	}

	var warnings []string

	// Check hadith number sequence
	var prevNum int
	for _, e := range page.Entries {
		if e.Number == nil || e.IsContinuation {
			continue
		}
		num := *e.Number
		if prevNum > 0 && num != prevNum+1 {
			warnings = append(warnings, fmt.Sprintf("entry number gap: %d → %d", prevNum, num))
			v.HadithNumberSequenceValid = false
		}
		prevNum = num
	}

	// Check for entries without type
	for i, e := range page.Entries {
		if e.Type == "" {
			warnings = append(warnings, fmt.Sprintf("entry %d has no type", i))
		}
	}

	// Check for empty arabic text
	for i, e := range page.Entries {
		if e.ArabicText == "" && !e.IsContinuation {
			warnings = append(warnings, fmt.Sprintf("entry %d has empty arabic_text", i))
		}
	}

	if len(warnings) > 0 {
		v.Status = "warnings"
		v.Warnings = warnings
	}

	return v
}
