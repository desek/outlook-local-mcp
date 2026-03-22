package validate

import (
	"strings"
	"testing"
)

func TestValidateDatetime_Valid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"bare datetime", "2026-04-15T09:00:00"},
		{"datetime with Z", "2026-04-15T09:00:00Z"},
		{"RFC3339", "2026-04-15T09:00:00+05:00"},
		{"RFC3339 negative offset", "2026-04-15T09:00:00-04:00"},
		{"milliseconds with Z", "2026-04-15T09:00:00.000Z"},
		{"milliseconds with offset", "2026-04-15T09:00:00.000+05:00"},
		{"midnight", "2026-01-01T00:00:00"},
		{"end of day", "2026-12-31T23:59:59Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateDatetime(tc.value, "test_param"); err != nil {
				t.Errorf("expected no error for %q, got: %v", tc.value, err)
			}
		})
	}
}

func TestValidateDatetime_Invalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"empty string", ""},
		{"date only", "2026-04-15"},
		{"time only", "09:00:00"},
		{"invalid format", "not-a-date"},
		{"wrong separator", "2026/04/15T09:00:00"},
		{"missing time", "2026-04-15T"},
		{"spaces", "2026-04-15 09:00:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateDatetime(tc.value, "start_datetime")
			if err == nil {
				t.Errorf("expected error for %q, got nil", tc.value)
				return
			}
			if !strings.Contains(err.Error(), "start_datetime") {
				t.Errorf("error should contain param name 'start_datetime', got: %v", err)
			}
			if !strings.Contains(err.Error(), "ISO 8601") {
				t.Errorf("error should mention ISO 8601, got: %v", err)
			}
		})
	}
}

func TestValidateTimezone_Valid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"UTC",
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"US/Pacific",
	}
	for _, tz := range cases {
		t.Run(tz, func(t *testing.T) {
			t.Parallel()
			if err := ValidateTimezone(tz, "timezone"); err != nil {
				t.Errorf("expected no error for %q, got: %v", tz, err)
			}
		})
	}
}

func TestValidateTimezone_Invalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"NotATimezone",
		"America/FakeCity",
		"12345",
	}
	for _, tz := range cases {
		t.Run(tz, func(t *testing.T) {
			t.Parallel()
			err := ValidateTimezone(tz, "start_timezone")
			if err == nil {
				t.Errorf("expected error for %q, got nil", tz)
				return
			}
			if !strings.Contains(err.Error(), "start_timezone") {
				t.Errorf("error should contain param name, got: %v", err)
			}
		})
	}
}

func TestValidateEmail_Valid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"user@example.com",
		"first.last@domain.org",
		"user+tag@example.com",
		"<user@example.com>",
	}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			t.Parallel()
			if err := ValidateEmail(email); err != nil {
				t.Errorf("expected no error for %q, got: %v", email, err)
			}
		})
	}
}

func TestValidateEmail_Invalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"not-an-email",
		"@missing-local.com",
		"missing-at-sign",
	}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			t.Parallel()
			err := ValidateEmail(email)
			if err == nil {
				t.Errorf("expected error for %q, got nil", email)
				return
			}
			if !strings.Contains(err.Error(), "invalid email") {
				t.Errorf("error should mention 'invalid email', got: %v", err)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	t.Parallel()

	t.Run("within limit", func(t *testing.T) {
		t.Parallel()
		if err := ValidateStringLength("hello", "subject", 255); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("at limit", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("a", 255)
		if err := ValidateStringLength(s, "subject", 255); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("a", 256)
		err := ValidateStringLength(s, "subject", 255)
		if err == nil {
			t.Error("expected error for string over limit")
			return
		}
		if !strings.Contains(err.Error(), "subject") {
			t.Errorf("error should contain param name, got: %v", err)
		}
		if !strings.Contains(err.Error(), "255") {
			t.Errorf("error should contain max length, got: %v", err)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		if err := ValidateStringLength("", "subject", 255); err != nil {
			t.Errorf("unexpected error for empty string: %v", err)
		}
	})

	t.Run("unicode", func(t *testing.T) {
		t.Parallel()
		// Unicode characters may be multi-byte but len() counts bytes.
		s := strings.Repeat("\U0001f600", 100) // 100 emoji = 400 bytes
		err := ValidateStringLength(s, "subject", 255)
		if err == nil {
			t.Error("expected error for unicode string exceeding byte limit")
		}
	})
}

func TestValidateResourceID(t *testing.T) {
	t.Parallel()

	t.Run("valid ID", func(t *testing.T) {
		t.Parallel()
		if err := ValidateResourceID("AAMkAGI2TGuLAAA=", "event_id"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		t.Parallel()
		err := ValidateResourceID("", "event_id")
		if err == nil {
			t.Error("expected error for empty ID")
			return
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Errorf("error should mention 'must not be empty', got: %v", err)
		}
	})

	t.Run("ID too long", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("x", 513)
		err := ValidateResourceID(long, "calendar_id")
		if err == nil {
			t.Error("expected error for ID exceeding max length")
			return
		}
		if !strings.Contains(err.Error(), "512") {
			t.Errorf("error should mention max length 512, got: %v", err)
		}
	})

	t.Run("ID at max length", func(t *testing.T) {
		t.Parallel()
		s := strings.Repeat("x", 512)
		if err := ValidateResourceID(s, "event_id"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidateImportance(t *testing.T) {
	t.Parallel()

	valid := []string{"low", "normal", "high", "Low", "NORMAL", "High", "LOW"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			t.Parallel()
			if err := ValidateImportance(v); err != nil {
				t.Errorf("expected no error for %q, got: %v", v, err)
			}
		})
	}

	invalid := []string{"", "critical", "medium", "urgent"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			t.Parallel()
			err := ValidateImportance(v)
			if err == nil {
				t.Errorf("expected error for %q, got nil", v)
				return
			}
			if !strings.Contains(err.Error(), "importance") {
				t.Errorf("error should mention 'importance', got: %v", err)
			}
		})
	}
}

func TestValidateSensitivity(t *testing.T) {
	t.Parallel()

	valid := []string{"normal", "personal", "private", "confidential", "Normal", "PRIVATE", "Confidential"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			t.Parallel()
			if err := ValidateSensitivity(v); err != nil {
				t.Errorf("expected no error for %q, got: %v", v, err)
			}
		})
	}

	invalid := []string{"", "secret", "public", "internal"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			t.Parallel()
			err := ValidateSensitivity(v)
			if err == nil {
				t.Errorf("expected error for %q, got nil", v)
				return
			}
			if !strings.Contains(err.Error(), "sensitivity") {
				t.Errorf("error should mention 'sensitivity', got: %v", err)
			}
		})
	}
}

func TestValidateShowAs(t *testing.T) {
	t.Parallel()

	valid := []string{"free", "tentative", "busy", "oof", "workingElsewhere", "Free", "BUSY", "WorkingElsewhere"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			t.Parallel()
			if err := ValidateShowAs(v); err != nil {
				t.Errorf("expected no error for %q, got: %v", v, err)
			}
		})
	}

	invalid := []string{"", "available", "unavailable", "away"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			t.Parallel()
			err := ValidateShowAs(v)
			if err == nil {
				t.Errorf("expected error for %q, got nil", v)
				return
			}
			if !strings.Contains(err.Error(), "show_as") {
				t.Errorf("error should mention 'show_as', got: %v", err)
			}
		})
	}
}

func TestValidateAttendeeType(t *testing.T) {
	t.Parallel()

	valid := []string{"required", "optional", "resource", "Required", "OPTIONAL", "Resource"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			t.Parallel()
			if err := ValidateAttendeeType(v); err != nil {
				t.Errorf("expected no error for %q, got: %v", v, err)
			}
		})
	}

	invalid := []string{"", "mandatory", "cc", "bcc"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			t.Parallel()
			err := ValidateAttendeeType(v)
			if err == nil {
				t.Errorf("expected error for %q, got nil", v)
				return
			}
			if !strings.Contains(err.Error(), "attendee type") {
				t.Errorf("error should mention 'attendee type', got: %v", err)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"single char truncation", "ab", 1, "a..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Truncate(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}
