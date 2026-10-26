package web

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var tokenDecl = regexp.MustCompile(`(--[a-z-]+):(#[0-9a-fA-F]{3,6})`)

func cssTokens(block string) map[string]string {
	out := map[string]string{}
	for _, m := range tokenDecl.FindAllStringSubmatch(block, -1) {
		out[m[1]] = strings.ToLower(m[2])
	}
	return out
}

func luminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		x := float64(v) / 255
		if x <= 0.03928 {
			c[i] = x / 12.92
		} else {
			c[i] = math.Pow((x+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// T-200: Storm Blue is the primary colour, status colours are tokens only,
// and every text token meets WCAG 2.2 AA on the surface it is used on.
func TestStormBluePaletteAndContrast(t *testing.T) {
	raw, err := os.ReadFile("static/crab.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	darkAt := strings.Index(css, "prefers-color-scheme:dark")
	if darkAt < 0 {
		t.Fatal("dark theme missing")
	}
	light, dark := cssTokens(css[:darkAt]), cssTokens(css[darkAt:strings.Index(css, "@media print")])

	locked := map[string]string{
		"--gas-accent": "#334a5e", "--gas-text": "#17212b", "--gas-text-muted": "#687786",
		"--gas-border": "#e7eaec", "--crab-page": "#f7f7f5",
		"--gas-success": "#2e7d5a", "--gas-warning": "#c6922d", "--gas-danger": "#b74b4b",
	}
	for token, want := range locked {
		if light[token] != want {
			t.Errorf("light %s=%s, locked value %s", token, light[token], want)
		}
	}
	// Green, amber and red appear only as status tokens.
	for _, status := range []string{"#2e7d5a", "#c6922d", "#b74b4b"} {
		if n := strings.Count(strings.ToLower(css), status); n != 1 {
			t.Errorf("status colour %s used %d times; status tokens only", status, n)
		}
	}
	for name, theme := range map[string]map[string]string{"light": light, "dark": dark} {
		pairs := []struct {
			fg, bg string
			min    float64
		}{
			{"--gas-text", "--gas-surface", 4.5},
			{"--gas-text", "--gas-surface-raised", 4.5},
			{"--gas-text", "--crab-page", 4.5},
			{"--gas-text-muted", "--gas-surface", 4.5},
			{"--gas-accent", "--gas-surface", 4.5},
			{"--gas-on-accent", "--gas-accent", 4.5},
			{"--gas-danger", "--gas-surface", 4.5},
			{"--crab-line", "--gas-surface", 3},
			{"--gas-focus", "--gas-surface", 3},
		}
		for _, p := range pairs {
			fg, bg := theme[p.fg], theme[p.bg]
			if fg == "" || bg == "" {
				t.Errorf("%s: missing %s or %s", name, p.fg, p.bg)
				continue
			}
			if got := contrast(fg, bg); got < p.min {
				t.Errorf("%s: %s %s on %s %s contrast %.2f < %.1f", name, p.fg, fg, p.bg, bg, got, p.min)
			}
		}
	}
}
