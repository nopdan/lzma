package lzma

import "testing"

func TestStateTransitions(t *testing.T) {
	if updateStateLiteral(0) != 0 {
		t.Fatalf("literal from 0 should stay 0")
	}
	if updateStateLiteral(4) != 1 {
		t.Fatalf("literal from 4 should be 1")
	}
	if updateStateLiteral(10) != 4 {
		t.Fatalf("literal from 10 should be 4")
	}

	if updateStateMatch(0) != 7 || updateStateMatch(8) != 10 {
		t.Fatalf("unexpected match transition")
	}
	if updateStateRep(0) != 8 || updateStateRep(8) != 11 {
		t.Fatalf("unexpected rep transition")
	}
	if updateStateShortRep(0) != 9 || updateStateShortRep(8) != 11 {
		t.Fatalf("unexpected short rep transition")
	}
}

func TestGetLenToPosState(t *testing.T) {
	cases := []struct {
		length uint32
		want   int
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
