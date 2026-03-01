package lzma

import "testing"

func TestStateTransitions(t *testing.T) {
	tests := []struct {
		name string
		got  uint32
		want uint32
	}{
		{name: "literal-0", got: updateStateLiteral(0), want: 0},
		{name: "literal-4", got: updateStateLiteral(4), want: 1},
		{name: "literal-10", got: updateStateLiteral(10), want: 4},
		{name: "match-0", got: updateStateMatch(0), want: 7},
		{name: "match-8", got: updateStateMatch(8), want: 10},
		{name: "rep-0", got: updateStateRep(0), want: 8},
		{name: "rep-8", got: updateStateRep(8), want: 11},
		{name: "short-rep-0", got: updateStateShortRep(0), want: 9},
		{name: "short-rep-8", got: updateStateShortRep(8), want: 11},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestGetLenToPosState(t *testing.T) {
	cases := []struct {
		length uint32
		want   uint32
	}{
		{2, 0},
		{3, 1},
		{4, 2},
		{5, 3},
		{6, 3},
	}

	for _, tc := range cases {
		if got := getLenToPosState(tc.length); got != tc.want {
			t.Fatalf("getLenToPosState(%d) = %d, want %d", tc.length, got, tc.want)
		}
	}
}
