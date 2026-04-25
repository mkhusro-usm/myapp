package rule

import "testing"

func TestParseMode(t *testing.T) {
	t.Run("evaluate", func(t *testing.T) {
		m, err := ParseMode("evaluate")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m != ModeEvaluate {
			t.Errorf("mode = %q, want %q", m, ModeEvaluate)
		}
	})

	t.Run("apply", func(t *testing.T) {
		m, err := ParseMode("apply")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m != ModeApply {
			t.Errorf("mode = %q, want %q", m, ModeApply)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := ParseMode("destroy")
		if err == nil {
			t.Fatal("expected error for invalid mode")
		}
	})
}
