package editor

import "testing"

func TestResetLeavesVisualMode(t *testing.T) {
	e := run(t, "one\ntwo\nthree", "vj")
	if e.Mode() != ModeVisual {
		t.Fatalf("Mode = %v, want visual before the reset", e.Mode())
	}

	e.Reset()

	if e.Mode() != ModeNormal {
		t.Fatalf("Mode = %v, want normal", e.Mode())
	}
}

func TestResetDropsTheVisualAnchor(t *testing.T) {
	e := run(t, "one\ntwo\nthree", "vj<esc>")
	e.Reset()

	feed(t, e, "gv")

	if e.Mode() != ModeNormal {
		t.Fatalf("Mode = %v, want normal. gv reselected an anchor from the old buffer", e.Mode())
	}
}

func TestResetClearsTheHalfTypedCommand(t *testing.T) {
	e := run(t, "one two three", "2d")
	if e.PendingKeys() == "" {
		t.Fatal("nothing pending before the reset")
	}

	e.Reset()
	if e.PendingKeys() != "" {
		t.Fatalf("PendingKeys = %q, want empty", e.PendingKeys())
	}

	// w now moves rather than completing the abandoned 2dw.
	feed(t, e, "w")
	if e.Text() != "one two three" {
		t.Fatalf("Text = %q, want unchanged. The pending operator fired", e.Text())
	}
	if e.Cursor() != (Pos{0, 4}) {
		t.Fatalf("Cursor = %v, want {0 4}", e.Cursor())
	}
}

func TestResetClearsTheCommandLine(t *testing.T) {
	e := run(t, "one", ":wq")
	if e.CommandLine() != ":wq" {
		t.Fatalf("CommandLine = %q, want :wq before the reset", e.CommandLine())
	}

	e.Reset()

	if e.Mode() != ModeNormal {
		t.Fatalf("Mode = %v, want normal", e.Mode())
	}
	if e.CommandLine() != "" {
		t.Fatalf("CommandLine = %q, want empty", e.CommandLine())
	}
	if e.QuitRequested() {
		t.Fatal("the abandoned :wq still ran")
	}
}

func TestResetClearsTheSearch(t *testing.T) {
	e := run(t, "one\ntwo\none", "/one<cr>")
	if e.SearchPattern() != "one" {
		t.Fatalf("SearchPattern = %q, want one before the reset", e.SearchPattern())
	}

	e.Reset()

	if e.SearchPattern() != "" {
		t.Fatalf("SearchPattern = %q, want empty", e.SearchPattern())
	}

	// n has nothing to repeat, so the cursor stays put.
	e.SetCursor(Pos{})
	feed(t, e, "n")
	if e.Cursor() != (Pos{}) {
		t.Fatalf("Cursor = %v, want the start. n repeated a cleared search", e.Cursor())
	}
}

func TestResetKeepsTheText(t *testing.T) {
	e := run(t, "one\ntwo", "x")
	e.Reset()
	if e.Text() != "ne\ntwo" {
		t.Fatalf("Text = %q, want ne\\ntwo", e.Text())
	}
}
