package mboxlib

import "testing"

func TestCountLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 1},
		{"\n", 1},
		{"abc\n", 1},
		{"\n\n", 2},
		{"abc\ndef", 2},
		{"abc\ndef\n", 2},
		{"abc\ndef\nghi", 3},
		{"abc\n\nghi", 3},
		{"abc\ndef\nghi\n", 3},
		{"abc\ndef\nghi\n\n", 4},
		{"abc\ndef\nghi\njkl\n", 4},
		{"\ndef\nghi\njkl\n", 4},
		{"abc\n\nghi\njkl\n", 4},
		{"abc\ndef\n\njkl\n", 4},
		{"abc\ndef\nghi\n\n", 4},
		{"abc\ndef\n\njkl\nmno", 5},
		{"abc\ndef\n\njkl\nmno\n", 5},
	}
	for _, tc := range tests {
		if got := countLines([]byte(tc.input)); got != tc.want {
			t.Errorf("countLines(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
