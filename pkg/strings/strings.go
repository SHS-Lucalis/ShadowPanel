package strings

func IsNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// IsSlug checks if the string contains only lowercase letters, digits, underscores, and hyphens.
func IsSlug(s string) bool {
	for _, c := range s {
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isSeparator := c == '_' || c == '-'

		if !isLower && !isDigit && !isSeparator {
			return false
		}
	}

	return true
}
