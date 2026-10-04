package main

import "testing"

func TestNormalizeClientPhoneLebanon(t *testing.T) {
	valid := map[string]string{
		"78000111":        "+96178000111",
		"79000111":        "+96179000111",
		"71000111":        "+96171000111",
		"81000111":        "+96181000111",
		"03000111":        "+9613000111",
		"3000111":         "+9613000111",
		"01000111":        "+9611000111",
		"07000111":        "+9617000111",
		"+961 78 000 111": "+96178000111",
		"0096179000111":   "+96179000111",
		"00000000":        "00000000",
	}
	for in, want := range valid {
		got, err := normalizeClientPhone(in)
		if err != nil || got != want {
			t.Errorf("normalizeClientPhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"80000111", "02000111", "7555", "77000111"} {
		if got, err := normalizeClientPhone(in); err == nil {
			t.Errorf("normalizeClientPhone(%q) = %q, want error", in, got)
		}
	}
}
