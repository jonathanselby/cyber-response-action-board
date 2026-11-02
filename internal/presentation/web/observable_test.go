package web

import (
	"strings"
	"testing"
)

// T-231 (presentation): an indicator value is rendered as escaped text, never
// as a link, and related incidents come only from the runtime.
func TestIndicatorsRenderAsTextNotLinks(t *testing.T) {
	body := get(t, withRuntime(t, &fakeRuntime{bootstrapped: true}), "/incident/"+testIncident+"/evidence").Body.String()
	for _, want := range []string{"Indicators", `<code class="crab-indicator">https://evil.example/login?a=1&amp;b=&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;</code>`, "SUSPECTED", "version 2", "(untrusted)", "INC-2", "Add indicator"} {
		if !strings.Contains(body, want) {
			t.Errorf("evidence page missing %q", want)
		}
	}
	if strings.Contains(body, `href="https://evil.example`) || strings.Contains(body, "<script>alert(1)") {
		t.Fatal("indicator value rendered as a link or markup")
	}
}
