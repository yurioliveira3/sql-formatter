package pipeline_test

import (
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

// ── FixIsNotNull ──────────────────────────────────────────────────────────────

func TestFixIsNotNull(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple column", "NOT col IS NULL", "col IS NOT NULL"},
		{"qualified column", "NOT t.col IS NULL", "t.col IS NOT NULL"},
		{"quoted column with space", `NOT nfr."Nº Order" IS NULL`, `nfr."Nº Order" IS NOT NULL`},
		{"multiple occurrences", "NOT a IS NULL AND NOT b IS NULL", "a IS NOT NULL AND b IS NOT NULL"},
		{"already correct unchanged", "col IS NOT NULL", "col IS NOT NULL"},
		{"no change needed", "WHERE active = 1", "WHERE active = 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.FixIsNotNull(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── RemoveTableAliasAs ────────────────────────────────────────────────────────

func TestRemoveTableAliasAs(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "FROM table AS alias",
			input: "FROM users AS u",
			want:  "FROM users u",
		},
		{
			name:  "JOIN table AS alias",
			input: "JOIN orders AS o ON u.id = o.user_id",
			want:  "JOIN orders o ON u.id = o.user_id",
		},
		{
			name:  "LEFT JOIN table AS alias",
			input: "LEFT JOIN products AS p ON o.product_id = p.id",
			want:  "LEFT JOIN products p ON o.product_id = p.id",
		},
		{
			name:  "subquery alias: ) AS alias",
			input: ") AS sub",
			want:  ") sub",
		},
		{
			name:  "SELECT col AS alias preserved",
			input: "SELECT name AS full_name FROM t",
			want:  "SELECT name AS full_name FROM t",
		},
		{
			name:  "no AS unchanged",
			input: "FROM users u",
			want:  "FROM users u",
		},
		{
			name:  "multi-line: only table alias lines changed",
			input: "SELECT name AS full_name\nFROM users AS u\nJOIN orders AS o ON u.id = o.id",
			want:  "SELECT name AS full_name\nFROM users u\nJOIN orders o ON u.id = o.id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.RemoveTableAliasAs(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── MergeFilterClauses ────────────────────────────────────────────────────────

func TestMergeFilterClauses(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "FILTER já em uma linha não muda",
			input: "COUNT(*) FILTER(WHERE active = 1)",
			want:  "COUNT(*) FILTER(WHERE active = 1)",
		},
		{
			name: "FILTER quebrado em múltiplas linhas é reunido",
			input: "COUNT(*) FILTER(\n  WHERE active = 1\n)",
			want:  "COUNT(*) FILTER( WHERE active = 1 )",
		},
		{
			name: "FILTER com múltiplas condições reunido",
			input: "COUNT(*) FILTER(\n  WHERE active = 1\n  AND type = 'A'\n)",
			want:  "COUNT(*) FILTER( WHERE active = 1 AND type = 'A' )",
		},
		{
			name:  "linha sem FILTER não muda",
			input: "SELECT id FROM t WHERE active = 1",
			want:  "SELECT id FROM t WHERE active = 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.MergeFilterClauses(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── SplitTopLevelAndOr ────────────────────────────────────────────────────────

func TestSplitTopLevelAndOr(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "nenhum AND/OR retorna slice com a linha original",
			input: "WHERE active = 1",
			want:  []string{"WHERE active = 1"},
		},
		{
			name:  "AND de nível superior é dividido",
			input: "WHERE a = 1 AND b = 2",
			want:  []string{"WHERE a = 1", "AND b = 2"},
		},
		{
			name:  "OR de nível superior é dividido",
			input: "WHERE a = 1 OR b = 2",
			want:  []string{"WHERE a = 1", "OR b = 2"},
		},
		{
			name:  "AND dentro de parênteses NÃO é dividido",
			input: "WHERE (a = 1 AND b = 2) AND c = 3",
			want:  []string{"WHERE (a = 1 AND b = 2)", "AND c = 3"},
		},
		{
			name:  "AND dentro de string NÃO é dividido",
			input: "WHERE col = 'FILTRAR POR UM LOTE' AND other = 1",
			want:  []string{"WHERE col = 'FILTRAR POR UM LOTE'", "AND other = 1"},
		},
		{
			name:  "OR embutido em palavra (POR) não é dividido",
			input: "WHERE col = 'FILTRAR POR UM LOTE'",
			want:  []string{"WHERE col = 'FILTRAR POR UM LOTE'"},
		},
		{
			name:  "múltiplos AND encadeados",
			input: "WHERE a = 1 AND b = 2 AND c = 3",
			want:  []string{"WHERE a = 1", "AND b = 2", "AND c = 3"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.SplitTopLevelAndOr(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("len=%d, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parte[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ── ApplySelectLayout ────────────────────────────────────────────────────────

func TestApplySelectLayout(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "múltiplas colunas ficam cada uma na sua linha",
			input: "SELECT id, name, email",
			want:  "SELECT\n   id,\n   name,\n   email",
		},
		{
			name:  "coluna única permanece na mesma linha",
			input: "SELECT id",
			want:  "SELECT id",
		},
		{
			name:  "SELECT * permanece na mesma linha",
			input: "SELECT *",
			want:  "SELECT *",
		},
		{
			name:  "funções com vírgula interna não são quebradas no lugar errado",
			input: "SELECT COUNT(*), SUM(total)",
			want:  "SELECT\n   COUNT(*),\n   SUM(total)",
		},
		{
			name:  "SELECT já sozinho não é alterado",
			input: "SELECT",
			want:  "SELECT",
		},
		{
			name:  "SELECT sozinho seguido de múltiplas colunas na próxima linha",
			input: "SELECT\nCOUNT(*), name",
			want:  "SELECT\n   COUNT(*),\n   name",
		},
		{
			name:  "SELECT sozinho com coluna única não é alterado",
			input: "SELECT\nid",
			want:  "SELECT\nid",
		},
		{
			name:  "indentação do SELECT é preservada",
			input: "  SELECT id, name",
			want:  "  SELECT\n     id,\n     name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.ApplySelectLayout(tc.input)
			if got != tc.want {
				t.Errorf("\ngot:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// ── ApplyWhereLayout ──────────────────────────────────────────────────────────

func TestApplyWhereLayout(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "WHERE com condição vai para linha indentada",
			input: "WHERE active = 1",
			want:  "WHERE\n   active = 1",
		},
		{
			name:  "AND após WHERE é indentado junto",
			input: "WHERE active = 1\nAND type = 'A'",
			want:  "WHERE\n   active = 1\n   AND type = 'A'",
		},
		{
			name:  "OR após WHERE é indentado junto",
			input: "WHERE active = 1\nOR type = 'A'",
			want:  "WHERE\n   active = 1\n   OR type = 'A'",
		},
		{
			name:  "múltiplos AND/OR todos indentados",
			input: "WHERE a = 1\nAND b = 2\nAND c = 3",
			want:  "WHERE\n   a = 1\n   AND b = 2\n   AND c = 3",
		},
		{
			name:  "HAVING segue o mesmo padrão",
			input: "HAVING COUNT(*) > 1\nAND SUM(x) > 0",
			want:  "HAVING\n   COUNT(*) > 1\n   AND SUM(x) > 0",
		},
		{
			name:  "WHERE já sozinho não é alterado",
			input: "WHERE",
			want:  "WHERE",
		},
		{
			name:  "linha após WHERE que não é AND/OR interrompe o bloco",
			input: "WHERE active = 1\nFROM t",
			want:  "WHERE\n   active = 1\nFROM t",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.ApplyWhereLayout(tc.input)
			if got != tc.want {
				t.Errorf("\ngot:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// ── ApplyAndOrLayout ──────────────────────────────────────────────────────────

func TestApplyAndOrLayout(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "linha sem AND/OR não muda",
			input: "WHERE active = 1",
			want:  "WHERE active = 1",
		},
		{
			name:  "AND quebra em linhas separadas",
			input: "WHERE a = 1 AND b = 2",
			want:  "WHERE a = 1\nAND b = 2",
		},
		{
			name:  "OR quebra em linhas separadas",
			input: "WHERE a = 1 OR b = 2",
			want:  "WHERE a = 1\nOR b = 2",
		},
		{
			name:  "comentário -- com AND não é quebrado",
			input: "WHERE\n  a = 1\n  -- AND b = 2",
			want:  "WHERE\n  a = 1\n  -- AND b = 2",
		},
		{
			name:  "comentário -- com OR não é quebrado",
			input: "WHERE\n  a = 1\n  -- OR b = 2",
			want:  "WHERE\n  a = 1\n  -- OR b = 2",
		},
		{
			name:  "indentação preservada",
			input: "  WHERE a = 1 AND b = 2",
			want:  "  WHERE a = 1\n  AND b = 2",
		},
		{
			name:  "POR dentro de string não divide",
			input: "  AND col = 'FILTRAR POR UM LOTE'",
			want:  "  AND col = 'FILTRAR POR UM LOTE'",
		},
		{
			name:  "múltiplas linhas processadas",
			input: "WHERE a = 1 AND b = 2\nAND c = 3 OR d = 4",
			want:  "WHERE a = 1\nAND b = 2\nAND c = 3\nOR d = 4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.ApplyAndOrLayout(tc.input)
			if got != tc.want {
				t.Errorf("\ngot:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}
