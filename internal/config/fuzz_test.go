// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"
)

func FuzzUnquote(f *testing.F) {
	f.Add(`"v"`)
	f.Add(`'v'`)
	f.Add(`"`)
	f.Add(`"mis'`)
	f.Add(``)
	f.Fuzz(func(t *testing.T, s string) {
		got := unquote(s) // must never panic
		if len(got) > len(s) {
			t.Errorf("unquote(%q) = %q grew the string", s, got)
		}
		if got != s && len(s)-len(got) != 2 {
			t.Errorf("unquote(%q) = %q must strip exactly one pair", s, got)
		}
	})
}

func FuzzSplitList(f *testing.F) {
	f.Add("a,b")
	f.Add(" A , ,b,")
	f.Add(",,,")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		out := splitList(s) // must never panic
		for _, item := range out {
			if item == "" {
				t.Errorf("splitList(%q) contains empty entry: %v", s, out)
			}
			if item != strings.ToLower(strings.TrimSpace(item)) {
				t.Errorf("splitList(%q) entry %q not normalized", s, item)
			}
			if strings.Contains(item, ",") {
				t.Errorf("splitList(%q) entry %q contains a comma", s, item)
			}
		}
	})
}
