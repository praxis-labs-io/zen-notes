package editor

import "testing"

func TestGXRequestsLinkFromAnywhereInItsSpan(t *testing.T) {
	const text = "[text](https://example.com)"
	tests := []struct {
		name string
		keys string
	}{
		{"opening bracket", "gx"},
		{"label", "llgx"},
		{"closing label bracket", "5lgx"},
		{"opening destination parenthesis", "6lgx"},
		{"destination", "10lgx"},
		{"closing parenthesis", "$gx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, text, tt.keys)
			target, ok := e.TakeOpenLinkRequest()
			if !ok || target != "https://example.com" {
				t.Fatalf("link request = %q, %v, want https://example.com", target, ok)
			}
			if e.Text() != text {
				t.Fatalf("Text = %q, want unchanged", e.Text())
			}
		})
	}
}

func TestGXUsesRunePositionsForUnicodeLabels(t *testing.T) {
	e := run(t, "前 [日本語](https://example.com) 後", "3lgx")
	target, ok := e.TakeOpenLinkRequest()
	if !ok || target != "https://example.com" {
		t.Fatalf("link request = %q, %v, want https://example.com", target, ok)
	}
}

func TestGXParsesMarkdownDestinations(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"balanced parentheses", "[docs](https://example.com/wiki/Foo_(bar))", "https://example.com/wiki/Foo_(bar)"},
		{"pointy brackets", "[site](<https://example.com>)", "https://example.com"},
		{"double quoted title", `[site](https://example.com "Site")`, "https://example.com"},
		{"single quoted title", "[site](https://example.com 'Site')", "https://example.com"},
		{"parenthesized title", "[site](https://example.com (Site))", "https://example.com"},
		{"escaped punctuation", `[docs](https://example.com/Foo_\(bar\))`, "https://example.com/Foo_(bar)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, tt.text, "$gx")
			target, ok := e.TakeOpenLinkRequest()
			if !ok || target != tt.want {
				t.Fatalf("link request = %q, %v, want %q", target, ok, tt.want)
			}
		})
	}
}

func TestGXSelectsTheLinkUnderTheCursor(t *testing.T) {
	e := run(t, "[a](https://a.example) [b](https://b.example)", "$gx")
	target, ok := e.TakeOpenLinkRequest()
	if !ok || target != "https://b.example" {
		t.Fatalf("link request = %q, %v, want https://b.example", target, ok)
	}
}

func TestGXRejectsTextAndMalformedLinks(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"outside a link", "plain [link](https://example.com)"},
		{"missing destination parenthesis", "[link](https://example.com"},
		{"missing label bracket", "[link(https://example.com)"},
		{"unbalanced destination", "[link](https://example.com/Foo_(bar)"},
		{"unterminated pointy destination", "[link](<https://example.com)"},
		{"unterminated title", `[link](https://example.com "Site)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, tt.text, "gx")
			if target, ok := e.TakeOpenLinkRequest(); ok {
				t.Fatalf("unexpected link request %q", target)
			}
			if e.Message() != "no link under cursor" {
				t.Fatalf("Message = %q, want no link under cursor", e.Message())
			}
			if e.PendingKeys() != "" {
				t.Fatalf("PendingKeys = %q, want cleared", e.PendingKeys())
			}
		})
	}
}

func TestGXPassesTargetValidationToTheApp(t *testing.T) {
	e := run(t, "[mail](mailto:hello@example.com)", "gx")
	target, ok := e.TakeOpenLinkRequest()
	if !ok || target != "mailto:hello@example.com" {
		t.Fatalf("link request = %q, %v, want mailto target", target, ok)
	}
}

func TestGXIsOnlyANormalModeCommand(t *testing.T) {
	tests := []struct {
		name string
		keys string
	}{
		{"visual mode", "vgx"},
		{"operator pending", "dgx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, "[link](https://example.com)", tt.keys)
			if target, ok := e.TakeOpenLinkRequest(); ok {
				t.Fatalf("unexpected link request %q", target)
			}
		})
	}
}

func TestTakingOpenLinkRequestClearsIt(t *testing.T) {
	e := run(t, "[link](https://example.com)", "gx")
	if _, ok := e.TakeOpenLinkRequest(); !ok {
		t.Fatal("first TakeOpenLinkRequest reported no request")
	}
	if target, ok := e.TakeOpenLinkRequest(); ok {
		t.Fatalf("second TakeOpenLinkRequest = %q, true, want cleared", target)
	}
}
