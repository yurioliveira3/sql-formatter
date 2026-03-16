package pipeline_test

// Testes de integração: pipeline completo chamando pipeline.Format() diretamente.

import (
	"strings"
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

// fmt executa o pipeline completo.
func fmt(sql string) string {
	return pipeline.Format(dedent(sql))
}

// dedent remove a indentação comum de todas as linhas (igual ao textwrap.dedent do Python)
func dedent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")

	// Remove linha vazia inicial (gerada pelo `` literal multilinha)
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	// Calcula indentação mínima (ignorando linhas em branco)
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent <= 0 {
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) >= minIndent {
			out = append(out, line[minIndent:])
		} else {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// ── TestSimpleSelect ──────────────────────────────────────────────────────────

func TestIntegration_SimpleSelect(t *testing.T) {
	t.Run("select single column", func(t *testing.T) {
		result := fmt("SELECT id FROM users")
		assertContains(t, result, "SELECT")
		assertContains(t, result, "FROM\n   users")
		assertEndsWith(t, result, ";")
	})

	t.Run("select uppercase keywords", func(t *testing.T) {
		result := fmt("select id, name from users")
		assertContains(t, result, "SELECT")
		assertContains(t, result, "FROM")
	})

	t.Run("select with where", func(t *testing.T) {
		result := fmt("SELECT id FROM users WHERE active = 1")
		assertContains(t, result, "WHERE")
		assertContains(t, result, "FROM\n   users")
	})

	t.Run("select with limit", func(t *testing.T) {
		result := fmt("SELECT id FROM users LIMIT 10")
		assertContains(t, result, "LIMIT\n   10")
	})

	t.Run("select with where and limit", func(t *testing.T) {
		result := fmt("SELECT id FROM users WHERE active = 1 LIMIT 5")
		assertContains(t, result, "WHERE")
		assertContains(t, result, "LIMIT\n   5")
	})
}

// ── TestJoins ────────────────────────────────────────────────────────────────

func TestIntegration_Joins(t *testing.T) {
	t.Run("inner join", func(t *testing.T) {
		result := fmt("SELECT u.id, o.total FROM users u INNER JOIN orders o ON u.id = o.user_id")
		assertContains(t, result, "FROM\n   users")
		assertContains(t, result, "INNER JOIN\n   orders")
	})

	t.Run("left join", func(t *testing.T) {
		result := fmt("SELECT u.id, o.total FROM users u LEFT JOIN orders o ON u.id = o.user_id")
		assertContains(t, result, "LEFT JOIN\n   orders")
	})

	t.Run("multiple joins", func(t *testing.T) {
		result := fmt(`
            SELECT u.id, o.total, p.name
            FROM users u
            INNER JOIN orders o ON u.id = o.user_id
            LEFT JOIN products p ON o.product_id = p.id
        `)
		assertContains(t, result, "FROM\n   users")
		assertContains(t, result, "INNER JOIN\n   orders")
		assertContains(t, result, "LEFT JOIN\n   products")
	})

	t.Run("right join", func(t *testing.T) {
		result := fmt("SELECT e.name, d.name FROM employees e RIGHT JOIN departments d ON e.dept_id = d.id")
		assertContains(t, result, "RIGHT JOIN\n   departments")
	})
}

// ── TestUpdate ────────────────────────────────────────────────────────────────

func TestIntegration_Update(t *testing.T) {
	t.Run("simple update", func(t *testing.T) {
		result := fmt("UPDATE users SET name = 'John' WHERE id = 1")
		assertContains(t, result, "UPDATE")
		assertContains(t, result, "SET")
		assertContains(t, result, "WHERE")
		assertEndsWith(t, result, ";")
	})

	t.Run("update multiple columns", func(t *testing.T) {
		result := fmt("UPDATE users SET name = 'John', email = 'john@test.com', active = 1 WHERE id = 42")
		assertContains(t, result, "UPDATE")
		assertContains(t, result, "SET")
		assertContains(t, result, "name = 'John'")
	})
}

// ── TestInsert ────────────────────────────────────────────────────────────────

func TestIntegration_Insert(t *testing.T) {
	t.Run("insert values", func(t *testing.T) {
		result := fmt("INSERT INTO users (name, email) VALUES ('John', 'john@test.com')")
		assertContains(t, result, "INSERT INTO")
		assertContains(t, result, "VALUES")
		assertEndsWith(t, result, ";")
	})

	t.Run("insert select", func(t *testing.T) {
		result := fmt("INSERT INTO archive_users SELECT * FROM users WHERE active = 0")
		assertContains(t, result, "INSERT INTO")
		assertContains(t, result, "SELECT")
		assertContains(t, result, "FROM\n   users")
	})
}

// ── TestCTE ───────────────────────────────────────────────────────────────────

func TestIntegration_CTE(t *testing.T) {
	t.Run("simple cte", func(t *testing.T) {
		result := fmt(`
            WITH active_users AS (
                SELECT id, name FROM users WHERE active = 1
            )
            SELECT * FROM active_users
        `)
		assertContains(t, result, "WITH")
		assertContains(t, result, "active_users")
		assertContains(t, result, "SELECT")
		assertEndsWith(t, result, ";")
	})

	t.Run("cte with join", func(t *testing.T) {
		result := fmt(`
            WITH recent_orders AS (
                SELECT user_id, total FROM orders WHERE created_at > '2024-01-01'
            )
            SELECT u.name, r.total
            FROM users u
            INNER JOIN recent_orders r ON u.id = r.user_id
        `)
		assertContains(t, result, "WITH")
		assertContains(t, result, "recent_orders")
		assertContains(t, result, "INNER JOIN\n   recent_orders")
	})

	t.Run("multiple ctes", func(t *testing.T) {
		result := fmt(`
            WITH
            active_users AS (
                SELECT id FROM users WHERE active = 1
            ),
            big_orders AS (
                SELECT user_id FROM orders WHERE total > 100
            )
            SELECT u.id
            FROM active_users u
            INNER JOIN big_orders o ON u.id = o.user_id
        `)
		assertContains(t, result, "active_users")
		assertContains(t, result, "big_orders")
	})
}

// ── TestMerge ────────────────────────────────────────────────────────────────

func TestIntegration_Merge(t *testing.T) {
	t.Run("matched and not matched", func(t *testing.T) {
		result := fmt(`
            MERGE INTO target_table a
            USING (
                SELECT col1, col2 FROM source_table b
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
                UPDATE SET a.col1 = b.col1
                WHERE 1 = 1
            WHEN NOT MATCHED THEN
                INSERT (col1) VALUES (b.col1)
        `)
		assertContains(t, result, "MERGE INTO\n\ttarget_table")
		assertContains(t, result, "WHEN MATCHED THEN")
		assertContains(t, result, "WHEN NOT MATCHED THEN")
		assertContains(t, result, "\tUPDATE SET")
		assertContains(t, result, "\tINSERT")
		assertEndsWith(t, result, ";")
	})

	t.Run("update only with where", func(t *testing.T) {
		result := fmt(`
            MERGE INTO employees a
            USING (
                SELECT id, salary FROM updates b
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
                UPDATE SET a.salary = b.salary
                WHERE 1 = 1
        `)
		assertContains(t, result, "MERGE INTO\n\temployees")
		assertContains(t, result, "WHEN MATCHED THEN")
		assertContains(t, result, "\tUPDATE SET")
		assertEndsWith(t, result, ";")
	})

	t.Run("using subquery has from layout", func(t *testing.T) {
		result := fmt(`
            MERGE INTO orders a
            USING (
                SELECT id, total
                FROM staging_orders b
                WHERE b.status = 'NEW'
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
                UPDATE SET a.total = b.total
                WHERE 1 = 1
        `)
		lines := strings.Split(result, "\n")
		fromIdx := -1
		for i, l := range lines {
			if strings.TrimSpace(l) == "FROM" {
				fromIdx = i
				break
			}
		}
		if fromIdx == -1 {
			t.Fatal("FROM não encontrado no resultado")
		}
		if !strings.Contains(lines[fromIdx+1], "staging_orders") {
			t.Errorf("esperava staging_orders na linha após FROM, got: %q", lines[fromIdx+1])
		}
	})

	t.Run("always ends with semicolon", func(t *testing.T) {
		result := fmt(`
            MERGE INTO t a
            USING (SELECT id FROM s b) b ON (a.id = b.id)
            WHEN MATCHED THEN UPDATE SET a.x = b.x WHERE 1 = 1
        `)
		assertEndsWith(t, result, ";")
	})
}

// ── TestSemicolonHandling ─────────────────────────────────────────────────────

func TestIntegration_SemicolonHandling(t *testing.T) {
	t.Run("trailing semicolon normalized", func(t *testing.T) {
		result := fmt("SELECT 1;")
		if strings.Count(result, ";") != 1 {
			t.Errorf("esperava 1 ponto-e-vírgula, got %d em: %q", strings.Count(result, ";"), result)
		}
		assertEndsWith(t, result, ";")
	})

	t.Run("multiple trailing semicolons normalized", func(t *testing.T) {
		result := fmt("SELECT 1;;;")
		if strings.Count(result, ";") != 1 {
			t.Errorf("esperava 1 ponto-e-vírgula, got %d", strings.Count(result, ";"))
		}
	})

	t.Run("output always ends with semicolon", func(t *testing.T) {
		result := fmt("SELECT id FROM users")
		assertEndsWith(t, result, ";")
	})
}

// ── TestSubquery ──────────────────────────────────────────────────────────────

func TestIntegration_Subquery(t *testing.T) {
	t.Run("subquery in where", func(t *testing.T) {
		result := fmt("SELECT id FROM users WHERE id IN (SELECT user_id FROM orders WHERE total > 100)")
		assertContains(t, result, "SELECT")
		assertContains(t, result, "FROM\n   users")
		assertEndsWith(t, result, ";")
	})

	t.Run("subquery in from", func(t *testing.T) {
		result := fmt("SELECT sub.id FROM (SELECT id FROM users WHERE active = 1) sub")
		assertContains(t, result, "SELECT")
		assertEndsWith(t, result, ";")
	})
}

// ── TestFixIsNotNull (integração) ─────────────────────────────────────────────

func TestIntegration_IsNotNull(t *testing.T) {
	t.Run("pipeline preserves IS NOT NULL", func(t *testing.T) {
		result := fmt("SELECT * FROM t WHERE t.col IS NOT NULL")
		assertContains(t, result, "IS NOT NULL")
		assertNotContains(t, result, "NOT t.col IS NULL")
	})
}

// ── TestAndOrLayout (integração) ─────────────────────────────────────────────

func TestIntegration_AndOrLayout(t *testing.T) {
	t.Run("commented AND preserved", func(t *testing.T) {
		result := fmt(`
            SELECT a FROM t
            WHERE x = 1
            --  AND y = 2
            --  AND z = 3
        `)
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "OR" || strings.HasPrefix(strings.TrimSpace(line), "OR ") {
				t.Errorf("OR isolado não deveria aparecer: %q", line)
			}
		}
		assertContains(t, result, "--  AND z = 3")
	})

	t.Run("comment content preserved exactly including extra spaces", func(t *testing.T) {
		result := fmt(`
            SELECT a FROM t
            WHERE x = 1
            --  AND y = 2
            --  AND z = 3
        `)
		// Dois espaços depois do -- devem ser preservados
		assertContains(t, result, "--  AND y = 2")
		assertContains(t, result, "--  AND z = 3")
		// A linha NÃO deve ser interpretada como AND real e quebrada
		assertNotContains(t, result, "\nAND y = 2")
		assertNotContains(t, result, "\nAND z = 3")
	})

	t.Run("POR inside string not split", func(t *testing.T) {
		result := fmt("SELECT * FROM t WHERE col = 'FILTRAR POR UM LOTE' AND other = 1")
		assertContains(t, result, "FILTRAR POR UM LOTE")
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "OR" || strings.HasPrefix(strings.TrimSpace(line), "OR ") {
				t.Errorf("OR isolado não deveria aparecer: %q", line)
			}
		}
	})
}

// ── TestIdempotency ───────────────────────────────────────────────────────────

// idempotent verifica que formatar duas vezes produz o mesmo resultado.
func idempotent(t *testing.T, sql string) {
	t.Helper()
	first := fmt(sql)
	second := pipeline.Format(first)
	if first != second {
		t.Errorf("não é idempotente:\n--- 1ª vez ---\n%s\n--- 2ª vez ---\n%s", first, second)
	}
}

func TestIdempotency(t *testing.T) {
	t.Run("SELECT com múltiplas colunas", func(t *testing.T) {
		idempotent(t, "SELECT id, name, email FROM users")
	})

	t.Run("SELECT com FILTER e coluna extra", func(t *testing.T) {
		idempotent(t, `
			SELECT
			   COUNT(DISTINCT o.id) FILTER (WHERE li.id IS NULL) AS orders_sem_line_items,
			   name
			FROM users
		`)
	})

	t.Run("ORDER BY simples", func(t *testing.T) {
		idempotent(t, "SELECT id FROM users ORDER BY name ASC")
	})

	t.Run("ORDER BY múltiplas colunas", func(t *testing.T) {
		idempotent(t, "SELECT id, name FROM users ORDER BY name ASC, id DESC")
	})

	t.Run("GROUP BY", func(t *testing.T) {
		idempotent(t, "SELECT status, COUNT(*) FROM orders GROUP BY status")
	})

	t.Run("LIMIT", func(t *testing.T) {
		idempotent(t, "SELECT id FROM users LIMIT 10")
	})

	t.Run("WHERE com AND", func(t *testing.T) {
		idempotent(t, "SELECT id FROM users WHERE active = 1 AND role = 'admin'")
	})

	t.Run("query completa do usuário", func(t *testing.T) {
		idempotent(t, `
			SELECT
			   COUNT(DISTINCT o.id) FILTER (WHERE li.id IS NULL) AS orders_sem_line_items,
			   name
			FROM users
			INNER JOIN orders ON users.id = orders.user_id
			JOIN orders ON users.id = orders.user_id
			LEFT JOIN items ON items.order_id = orders.id
			WHERE active = 1
			LIMIT 10
		`)
	})

	t.Run("ORDER BY com LIMIT", func(t *testing.T) {
		idempotent(t, "SELECT id, name FROM users WHERE active = 1 ORDER BY name ASC LIMIT 20")
	})

	t.Run("GROUP BY com HAVING", func(t *testing.T) {
		idempotent(t, "SELECT status, COUNT(*) cnt FROM orders GROUP BY status HAVING COUNT(*) > 5")
	})
}

// ── helpers de assert ─────────────────────────────────────────────────────────

func assertContains(t *testing.T, result, sub string) {
	t.Helper()
	if !strings.Contains(result, sub) {
		t.Errorf("resultado não contém %q\nresultado:\n%s", sub, result)
	}
}

func assertNotContains(t *testing.T, result, sub string) {
	t.Helper()
	if strings.Contains(result, sub) {
		t.Errorf("resultado não deveria conter %q\nresultado:\n%s", sub, result)
	}
}

func assertEndsWith(t *testing.T, result, suffix string) {
	t.Helper()
	trimmed := strings.TrimRight(result, "\n")
	if !strings.HasSuffix(trimmed, suffix) {
		t.Errorf("resultado deveria terminar com %q\nresultado:\n%s", suffix, result)
	}
}
