package main

import "testing"

func TestParseOpenArgs(t *testing.T) {
	cases := []struct {
		in        []string
		wantQuery string
		wantPrint bool
	}{
		{[]string{"定投"}, "定投", false},
		{[]string{"定投", "止盈"}, "定投 止盈", false},
		{[]string{"--print", "定投"}, "定投", true},
		{[]string{"定投", "--print"}, "定投", true},
		{[]string{"-p", "定投"}, "定投", true},
	}
	for _, c := range cases {
		gotQuery, gotPrint := parseOpenArgs(c.in)
		if gotQuery != c.wantQuery || gotPrint != c.wantPrint {
			t.Errorf("parseOpenArgs(%v) = (%q, %v), want (%q, %v)",
				c.in, gotQuery, gotPrint, c.wantQuery, c.wantPrint)
		}
	}
}
