package hold

import "testing"

func TestParseRefusesWhatItDoesNotKnow(t *testing.T) {
	for _, bad := range []string{"jump", "fire@0", "fire@soon", "left@-3"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("-hold %q was accepted", bad)
		}
	}
	if h, err := Parse(" fire , left@60 "); err != nil || len(h) != 2 || h[1].From != 60 || h[0].From != 1 {
		t.Errorf("-hold with spaces: %v %v", h, err)
	}
	if h, err := Parse(""); err != nil || len(h) != 0 {
		t.Errorf("an empty -hold: %v %v", h, err)
	}
}
