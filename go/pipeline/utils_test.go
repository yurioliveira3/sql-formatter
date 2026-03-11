package pipeline_test

import (
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

// ── StripTrailingSemicolons ───────────────────────────────────────────────────

func TestStripTrailingSemicolons(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"single semicolon", "SELECT 1;", "SELECT 1"},
		{"multiple semicolons", "SELECT 1;;;", "SELECT 1"},
		{"semicolon with trailing whitespace", "SELECT 1;   \n", "SELECT 1"},
		{"no semicolon", "SELECT 1", "SELECT 1"},
		{"semicolon in middle preserved", "SELECT 1; SELECT 2;", "SELECT 1; SELECT 2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.StripTrailingSemicolons(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── Finalize ──────────────────────────────────────────────────────────────────

func TestFinalize(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"adds semicolon on new line", "SELECT 1", "SELECT 1\n;\n"},
		{"normalizes CRLF", "SELECT 1\r\n", "SELECT 1\n;\n"},
		{"strips existing trailing semicolon before adding", "SELECT 1;", "SELECT 1\n;\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.Finalize(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── IsMerge ───────────────────────────────────────────────────────────────────

func TestIsMerge(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"merge uppercase", "MERGE INTO target", true},
		{"merge lowercase", "merge into target", true},
		{"merge with leading space", " MERGE INTO target", true}, // Python: \s*MERGE permite espaço inicial
		{"not merge - SELECT", "SELECT 1", false},
		{"not merge - UPDATE", "UPDATE t SET x=1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.IsMerge(tc.input)
			if got != tc.want {
				t.Errorf("IsMerge(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── ApplyOrderByLayout ────────────────────────────────────────────────────────

func TestApplyOrderByLayout(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"order by single column", "ORDER BY o.total DESC", "ORDER BY\n   o.total DESC"},
		{"order by multiple columns", "ORDER BY col1 ASC, col2 DESC", "ORDER BY\n   col1 ASC, col2 DESC"},
		{"group by single column", "GROUP BY col1", "GROUP BY\n   col1"},
		{"group by multiple columns", "GROUP BY col1, col2", "GROUP BY\n   col1, col2"},
		{"limit single value", "LIMIT 10", "LIMIT\n   10"},
		{"limit zero", "LIMIT 0", "LIMIT\n   0"},
		{"already split order by unchanged", "ORDER BY\n   col1", "ORDER BY\n   col1"},
		{"already split limit unchanged", "LIMIT\n   10", "LIMIT\n   10"},
		{"all three in same sql", "SELECT a\nGROUP BY a\nORDER BY a DESC\nLIMIT 5",
			"SELECT a\nGROUP BY\n   a\nORDER BY\n   a DESC\nLIMIT\n   5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.ApplyOrderByLayout(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
