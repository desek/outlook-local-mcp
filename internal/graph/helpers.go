package graph

// SafeStr returns the dereferenced value of a *string pointer, or an empty
// string if the pointer is nil. This prevents nil pointer dereference panics
// when accessing string fields from Microsoft Graph SDK model getters.
//
// Parameters:
//   - s: a pointer to a string value, which may be nil.
//
// Returns the string value pointed to by s, or "" if s is nil.
func SafeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// SafeBool returns the dereferenced value of a *bool pointer, or false if the
// pointer is nil. This prevents nil pointer dereference panics when accessing
// boolean fields from Microsoft Graph SDK model getters.
//
// Parameters:
//   - b: a pointer to a bool value, which may be nil.
//
// Returns the bool value pointed to by b, or false if b is nil.
func SafeBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// SafeInt32 returns the dereferenced value of an *int32 pointer, or 0 if the
// pointer is nil. This prevents nil pointer dereference panics when accessing
// int32 fields from Microsoft Graph SDK model getters.
//
// Parameters:
//   - i: a pointer to an int32 value, which may be nil.
//
// Returns the int32 value pointed to by i, or 0 if i is nil.
func SafeInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}
