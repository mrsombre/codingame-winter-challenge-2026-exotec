package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// bodyFacing — infer direction from body coordinates
// ---------------------------------------------------------------------------

// TestBodyFacing verifies direction inference from body segment positions.
// This is the entry point used by the main loop to determine each bot's
// facing before calling decision functions.
func TestBodyFacing(t *testing.T) {
	// Head right of neck → facing right.
	if f := bodyFacing([]Point{{4, 3}, {3, 3}}); f != DirRight {
		t.Errorf("right: got %d", f)
	}
	// Head above neck → facing up.
	if f := bodyFacing([]Point{{3, 2}, {3, 3}}); f != DirUp {
		t.Errorf("up: got %d", f)
	}
	// Single segment → DirUp default.
	if f := bodyFacing([]Point{{3, 3}}); f != DirUp {
		t.Errorf("single: got %d, want DirUp", f)
	}
}

// ---------------------------------------------------------------------------
// actionString — output formatting
// ---------------------------------------------------------------------------

// TestActionString verifies the output format "ID DIRECTION" which is
// sent to the game engine.  The reason suffix is only added in debug mode
// (which is off by default), so we verify it is NOT present.
func TestActionString(t *testing.T) {
	s := actionString(42, DirRight, "test")
	if !strings.HasPrefix(s, "42 RIGHT") {
		t.Errorf("got %q, want prefix '42 RIGHT'", s)
	}
	// debug is false → reason should not appear.
	if strings.Contains(s, "test") {
		t.Error("reason should not appear when debug=false")
	}
}

// TestActionStringAllDirections verifies correct direction names for all
// four cardinals.
func TestActionStringAllDirections(t *testing.T) {
	cases := []struct {
		dir  Direction
		want string
	}{
		{DirUp, "UP"},
		{DirRight, "RIGHT"},
		{DirDown, "DOWN"},
		{DirLeft, "LEFT"},
	}
	for _, c := range cases {
		s := actionString(0, c.dir, "")
		if !strings.Contains(s, c.want) {
			t.Errorf("dir %d: got %q, want to contain %q", c.dir, s, c.want)
		}
	}
}
