// Package ids validates identifiers coming from clients before they reach SQL.
package ids

// IsUUID accepts the canonical 8-4-4-4-12 hex form.
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
				return false
			}
		}
	}
	return true
}

// AllUUIDs reports whether every element is a UUID.
func AllUUIDs(values []string) bool {
	for _, v := range values {
		if !IsUUID(v) {
			return false
		}
	}
	return true
}
