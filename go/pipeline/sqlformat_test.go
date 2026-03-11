package pipeline_test

import (
	"strings"
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

// ── Keyword uppercasing ───────────────────────────────────────────────────────

func TestFormatSQL_Uppercasing(t *testing.T) {
	cases := []struct {
		name  string
		input string
		// want: substring que DEVE estar no resultado
		mustContain []string
		// want: substring que NÃO deve estar no resultado
		mustNotContain []string
	}{
		{
			name:        "lowercase keywords uppercased",
			input:       "select id from users where active = 1",
			mustContain: []string{"SELECT", "FROM", "WHERE"},
		},
		{
			name:        "mixed case keywords uppercased",
			input:       "Select Id From Users Where Active = 1",
			mustContain: []string{"SELECT", "FROM", "WHERE"},
		},
		{
			name:  "keywords inside string literal NOT uppercased",
			input: "SELECT 'select * from t' FROM users",
			// o conteúdo da string deve ser preservado
			mustContain:    []string{"SELECT", "'select * from t'"},
			mustNotContain: []string{"'SELECT * FROM T'"},
		},
		{
			name:  "keywords inside line comment NOT uppercased",
			input: "SELECT id -- select more columns\nFROM users",
			// o comentário deve ser preservado intacto
			mustContain:    []string{"-- select more columns"},
			mustNotContain: []string{"-- SELECT MORE COLUMNS"},
		},
		{
			name:  "keywords inside block comment NOT uppercased",
			input: "SELECT /*__KEEP_0__*/ id FROM users",
			// placeholders __KEEP__ devem sobreviver intactos
			mustContain: []string{"/*__KEEP_0__*/"},
		},
		{
			name:        "join variants uppercased",
			input:       "select a from t inner join u on t.id = u.id left join v on t.id = v.id",
			mustContain: []string{"INNER JOIN", "LEFT JOIN", "ON"},
		},
		{
			name:        "group by and order by uppercased",
			input:       "select a, count(*) from t group by a order by a desc",
			mustContain: []string{"GROUP BY", "ORDER BY", "DESC"},
		},
		{
			name:        "null and is uppercased",
			input:       "select * from t where col is null",
			mustContain: []string{"IS", "NULL"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.FormatSQL(tc.input)
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, got)
				}
			}
			for _, s := range tc.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output should not contain %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

// ── Estrutura por cláusula ────────────────────────────────────────────────────

func TestFormatSQL_ClauseBreaking(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		mustContain []string
	}{
		{
			name:        "SELECT and FROM on separate lines",
			input:       "SELECT id FROM users",
			mustContain: []string{"SELECT", "FROM"},
		},
		{
			name:        "WHERE on its own line",
			input:       "SELECT id FROM users WHERE active = 1",
			mustContain: []string{"FROM users\nWHERE", "WHERE active"},
		},
		{
			name:        "JOIN on its own line",
			input:       "SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id",
			mustContain: []string{"FROM users u\nJOIN"},
		},
		{
			name:        "INNER JOIN on its own line (not split between INNER and JOIN)",
			input:       "SELECT a FROM t INNER JOIN u ON t.id = u.id",
			mustContain: []string{"INNER JOIN"},
			// INNER e JOIN devem ficar na mesma linha, sem quebra entre eles
		},
		{
			name:        "LEFT JOIN stays together",
			input:       "SELECT a FROM t LEFT JOIN u ON t.id = u.id",
			mustContain: []string{"LEFT JOIN"},
		},
		{
			name:        "FULL OUTER JOIN stays together",
			input:       "SELECT a FROM t FULL OUTER JOIN u ON t.id = u.id",
			mustContain: []string{"FULL OUTER JOIN"},
		},
		{
			name:        "GROUP BY on its own line",
			input:       "SELECT a, COUNT(*) FROM t GROUP BY a",
			mustContain: []string{"FROM t\nGROUP BY"},
		},
		{
			name:        "ORDER BY on its own line",
			input:       "SELECT a FROM t ORDER BY a DESC",
			mustContain: []string{"FROM t\nORDER BY"},
		},
		{
			name:        "LIMIT on its own line",
			input:       "SELECT a FROM t LIMIT 10",
			mustContain: []string{"FROM t\nLIMIT"},
		},
		{
			name:        "HAVING on its own line",
			input:       "SELECT a, COUNT(*) FROM t GROUP BY a HAVING COUNT(*) > 1",
			mustContain: []string{"GROUP BY a\nHAVING"},
		},
		{
			name:        "UNION on its own line",
			input:       "SELECT a FROM t1 UNION SELECT a FROM t2",
			mustContain: []string{"UNION"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.FormatSQL(tc.input)
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

// ── Subqueries ficam em uma linha (depth > 0) ─────────────────────────────────

func TestFormatSQL_SubqueryNoBreak(t *testing.T) {
	cases := []struct {
		name           string
		input          string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:  "WHERE IN subquery stays on one line",
			input: "SELECT id FROM users WHERE id IN (SELECT user_id FROM orders WHERE total > 100)",
			// O SELECT/FROM/WHERE interno não deve causar quebra de linha
			mustContain: []string{"IN (SELECT user_id FROM orders WHERE total > 100)"},
		},
		{
			name:  "CTE subquery does not break inside parens",
			input: "WITH cte AS (SELECT id FROM t WHERE active = 1) SELECT * FROM cte",
			// conteúdo do CTE fica numa linha só
			mustContain: []string{"WITH cte AS (SELECT id FROM t WHERE active = 1)"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.FormatSQL(tc.input)
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, got)
				}
			}
			for _, s := range tc.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output should not contain %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

// ── Preservação de comentários ────────────────────────────────────────────────

func TestFormatSQL_CommentPreservation(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		mustContain []string
	}{
		{
			name:        "line comment preserved on same line",
			input:       "SELECT id, -- user id\nname FROM users",
			mustContain: []string{"-- user id"},
		},
		{
			name:        "block comment placeholder preserved",
			input:       "SELECT /*__KEEP_0__*/ id FROM users",
			mustContain: []string{"/*__KEEP_0__*/"},
		},
		{
			name:        "multiple placeholders preserved",
			input:       "SELECT /*__KEEP_0__*/ id, /*__KEEP_1__*/ name FROM users",
			mustContain: []string{"/*__KEEP_0__*/", "/*__KEEP_1__*/"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.FormatSQL(tc.input)
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}
