package openapi

import (
	"strings"
	"time"

	"github.com/open-mrp/apikit/pagination"
)

// sampleCursorTime is when the default documented cursor says its item was created, for an item without created_at or occurred_at.
var sampleCursorTime = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// documentationNextCursor is the cursor shown in a list example's next_page_url: the app's Examples.ListCursor, or a signed string cursor from the item's id and timestamp.
func documentationNextCursor(itemTypeName string, itemMap map[string]any) string {
	if active().Examples.ListCursor != nil {
		if c := active().Examples.ListCursor(itemTypeName, itemMap); c != "" {
			return c
		}
	}
	occurredAt := documentationOccurredAt(itemMap)
	if occurredAt.IsZero() {
		occurredAt = sampleCursorTime
	}
	id, _ := itemMap["id"].(string)
	if id == "" {
		return ""
	}
	return pagination.EncodeDocumentationStringCursor(occurredAt, id)
}

func documentationOccurredAt(itemMap map[string]any) time.Time {
	for _, key := range []string{"created_at", "occurred_at"} {
		if v, ok := itemMap[key]; ok {
			if t := parseDocumentationTime(v); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}

func parseDocumentationTime(v any) time.Time {
	switch val := v.(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t
		}
	case time.Time:
		return val
	}
	return time.Time{}
}

func isSignedPaginationCursor(cursor string) bool {
	return strings.Count(cursor, ".") == 1
}
