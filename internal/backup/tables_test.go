package backup

import (
	"reflect"
	"sort"
	"testing"
)

func TestDedup(t *testing.T) {
	got := dedup([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedup = %v, want %v", got, want)
	}
}

func TestTableSelectionValidate(t *testing.T) {
	cases := []struct {
		name    string
		sel     TableSelection
		wantErr bool
	}{
		{"vazio", TableSelection{}, true},
		{"all", TableSelection{All: true}, false},
		{"pattern", TableSelection{Pattern: "mdl_*"}, false},
		{"names", TableSelection{Names: []string{"users"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sel.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate erro=%v want=%v", err, tc.wantErr)
			}
		})
	}
}

func TestApplyPatternAndExcludes(t *testing.T) {
	all := []string{"mdl_user", "mdl_log", "logs_2024", "logs_old", "orders", "users"}

	got := applyFilters(all, "mdl_*", nil)
	sort.Strings(got)
	want := []string{"mdl_log", "mdl_user"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pattern mdl_* = %v, want %v", got, want)
	}

	got = applyFilters(all, "", []string{"logs_*", "mdl_*"})
	sort.Strings(got)
	want = []string{"orders", "users"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("excludes = %v, want %v", got, want)
	}

	got = applyFilters(all, "*", []string{"logs_*"})
	sort.Strings(got)
	want = []string{"mdl_log", "mdl_user", "orders", "users"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pattern + exclude = %v, want %v", got, want)
	}
}
