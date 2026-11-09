package store

import "testing"

func TestContainsPattern(t *testing.T) {
	for in, want := range map[string]string{
		"abc": "%abc%",
		"50%": "%50!%%",
		"a_b": "%a!_b%",
		"hi!": "%hi!!%",
		"%_!": "%!%!_!!%",
		`a\b`: `%a\b%`,
		"綠能":  "%綠能%",
		"":    "%%",
	} {
		if got := containsPattern(in); got != want {
			t.Errorf("containsPattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOrderBy(t *testing.T) {
	for f, want := range map[ListFilter]string{
		{}:                           "i.created_at ASC, i.id ASC",
		{Desc: true}:                 "i.created_at DESC, i.id DESC",
		{SortName: true}:             "i.name ASC, i.id ASC",
		{SortName: true, Desc: true}: "i.name DESC, i.id DESC",
	} {
		if got := f.orderBy(); got != want {
			t.Errorf("%+v: %q, want %q", f, got, want)
		}
	}
}
