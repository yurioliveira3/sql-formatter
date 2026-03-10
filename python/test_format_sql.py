"""
TDD test suite for format_sql_ansi.py
Run with: pytest test_format_sql.py -v
"""

import subprocess
import sys
import textwrap
from pathlib import Path

import pytest

# ── helpers ──────────────────────────────────────────────────────────────────

SCRIPT = Path(__file__).parent / "format_sql_ansi.py"


def fmt(sql: str) -> str:
    """Run the full formatter pipeline and return stdout."""
    result = subprocess.run(
        [sys.executable, str(SCRIPT)],
        input=textwrap.dedent(sql).strip(),
        text=True,
        capture_output=True,
    )
    assert result.returncode == 0, f"Formatter error:\n{result.stderr}"
    return result.stdout


# Import individual functions for unit testing
sys.path.insert(0, str(SCRIPT.parent))
from format_sql_ansi import (
    apply_and_or_layout,
    apply_from_join_layout,
    apply_limit_layout,
    apply_merge_layout,
    finalize,
    fix_is_not_null,
    strip_trailing_semicolons,
)


# ── unit: strip_trailing_semicolons ──────────────────────────────────────────


class TestStripTrailingSemicolons:
    def test_single_semicolon(self):
        assert strip_trailing_semicolons("SELECT 1;") == "SELECT 1"

    def test_multiple_semicolons(self):
        assert strip_trailing_semicolons("SELECT 1;;;") == "SELECT 1"

    def test_semicolon_with_trailing_whitespace(self):
        assert strip_trailing_semicolons("SELECT 1;   \n") == "SELECT 1"

    def test_no_semicolon(self):
        assert strip_trailing_semicolons("SELECT 1") == "SELECT 1"

    def test_semicolon_in_middle_is_preserved(self):
        sql = "SELECT 1; SELECT 2;"
        result = strip_trailing_semicolons(sql)
        assert result == "SELECT 1; SELECT 2"


# ── unit: apply_from_join_layout ─────────────────────────────────────────────


class TestFromJoinLayout:
    def test_from_moves_table_to_next_line(self):
        result = apply_from_join_layout("FROM users")
        assert result == "FROM\n\tusers"

    def test_join_moves_table_to_next_line(self):
        result = apply_from_join_layout("JOIN orders ON users.id = orders.user_id")
        assert result == "JOIN\n\torders ON users.id = orders.user_id"

    def test_left_join(self):
        result = apply_from_join_layout("LEFT JOIN orders ON users.id = orders.user_id")
        assert result == "LEFT JOIN\n\torders ON users.id = orders.user_id"

    def test_inner_join(self):
        result = apply_from_join_layout("INNER JOIN orders ON users.id = orders.user_id")
        assert result == "INNER JOIN\n\torders ON users.id = orders.user_id"

    def test_right_join(self):
        result = apply_from_join_layout("RIGHT JOIN dept ON e.dept_id = dept.id")
        assert result == "RIGHT JOIN\n\tdept ON e.dept_id = dept.id"

    def test_full_outer_join(self):
        result = apply_from_join_layout("FULL OUTER JOIN dept ON e.dept_id = dept.id")
        assert result == "FULL OUTER JOIN\n\tdept ON e.dept_id = dept.id"

    def test_from_preserves_indentation(self):
        result = apply_from_join_layout("  FROM sub_table")
        assert result == "  FROM\n  \tsub_table"

    def test_lone_from_keyword_untouched(self):
        result = apply_from_join_layout("FROM")
        assert result == "FROM"


# ── unit: apply_limit_layout ─────────────────────────────────────────────────


class TestLimitLayout:
    def test_limit_moves_value_to_next_line(self):
        result = apply_limit_layout("LIMIT 10")
        assert result == "LIMIT\n\t10"

    def test_limit_with_zero(self):
        result = apply_limit_layout("LIMIT 0")
        assert result == "LIMIT\n\t0"

    def test_already_split_limit_unchanged(self):
        # se já está em duas linhas, regex não bate
        sql = "LIMIT\n\t10"
        result = apply_limit_layout(sql)
        assert result == "LIMIT\n\t10"


# ── unit: finalize ───────────────────────────────────────────────────────────


class TestFinalize:
    def test_adds_semicolon_on_new_line(self):
        result = finalize("SELECT 1")
        assert result == "SELECT 1\n;\n"

    def test_normalizes_crlf(self):
        result = finalize("SELECT 1\r\n")
        assert result == "SELECT 1\n;\n"

    def test_strips_existing_trailing_semicolon_before_adding(self):
        result = finalize("SELECT 1;")
        assert result == "SELECT 1\n;\n"


# ── integration: full pipeline ───────────────────────────────────────────────


class TestSimpleSelect:
    def test_select_single_column(self):
        result = fmt("SELECT id FROM users")
        assert "SELECT" in result
        assert "FROM\n\tusers" in result
        assert result.strip().endswith(";")

    def test_select_uppercase_keywords(self):
        result = fmt("select id, name from users")
        assert "SELECT" in result
        assert "FROM" in result

    def test_select_with_where(self):
        result = fmt("SELECT id FROM users WHERE active = 1")
        assert "WHERE" in result
        assert "FROM\n\tusers" in result

    def test_select_with_limit(self):
        result = fmt("SELECT id FROM users LIMIT 10")
        assert "LIMIT\n\t10" in result

    def test_select_with_where_and_limit(self):
        result = fmt("SELECT id FROM users WHERE active = 1 LIMIT 5")
        assert "WHERE" in result
        assert "LIMIT\n\t5" in result


class TestJoins:
    def test_inner_join(self):
        sql = "SELECT u.id, o.total FROM users u INNER JOIN orders o ON u.id = o.user_id"
        result = fmt(sql)
        assert "FROM\n\tusers" in result
        assert "INNER JOIN\n\torders" in result

    def test_left_join(self):
        sql = "SELECT u.id, o.total FROM users u LEFT JOIN orders o ON u.id = o.user_id"
        result = fmt(sql)
        assert "LEFT JOIN\n\torders" in result

    def test_multiple_joins(self):
        sql = """
            SELECT u.id, o.total, p.name
            FROM users u
            INNER JOIN orders o ON u.id = o.user_id
            LEFT JOIN products p ON o.product_id = p.id
        """
        result = fmt(sql)
        assert "FROM\n\tusers" in result
        assert "INNER JOIN\n\torders" in result
        assert "LEFT JOIN\n\tproducts" in result

    def test_right_join(self):
        sql = "SELECT e.name, d.name FROM employees e RIGHT JOIN departments d ON e.dept_id = d.id"
        result = fmt(sql)
        assert "RIGHT JOIN\n\tdepartments" in result


class TestUpdate:
    def test_simple_update(self):
        result = fmt("UPDATE users SET name = 'John' WHERE id = 1")
        assert "UPDATE" in result
        assert "SET" in result
        assert "WHERE" in result
        assert result.strip().endswith(";")

    def test_update_multiple_columns(self):
        sql = "UPDATE users SET name = 'John', email = 'john@test.com', active = 1 WHERE id = 42"
        result = fmt(sql)
        assert "UPDATE" in result
        assert "SET" in result
        assert "name = 'John'" in result


class TestInsert:
    def test_insert_values(self):
        sql = "INSERT INTO users (name, email) VALUES ('John', 'john@test.com')"
        result = fmt(sql)
        assert "INSERT INTO" in result
        assert "VALUES" in result
        assert result.strip().endswith(";")

    def test_insert_select(self):
        sql = "INSERT INTO archive_users SELECT * FROM users WHERE active = 0"
        result = fmt(sql)
        assert "INSERT INTO" in result
        assert "SELECT" in result
        assert "FROM\n\tusers" in result


class TestCTE:
    def test_simple_cte(self):
        sql = """
            WITH active_users AS (
                SELECT id, name FROM users WHERE active = 1
            )
            SELECT * FROM active_users
        """
        result = fmt(sql)
        assert "WITH" in result
        assert "active_users" in result
        assert "SELECT" in result
        assert result.strip().endswith(";")

    def test_cte_with_join(self):
        sql = """
            WITH recent_orders AS (
                SELECT user_id, total FROM orders WHERE created_at > '2024-01-01'
            )
            SELECT u.name, r.total
            FROM users u
            INNER JOIN recent_orders r ON u.id = r.user_id
        """
        result = fmt(sql)
        assert "WITH" in result
        assert "recent_orders" in result
        assert "INNER JOIN\n\trecent_orders" in result

    def test_multiple_ctes(self):
        sql = """
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
        """
        result = fmt(sql)
        assert "active_users" in result
        assert "big_orders" in result


class TestMergeLayout:
    """Unit tests para apply_merge_layout — validam o formato linha a linha."""

    def test_merge_into_splits_table_to_next_line(self):
        sql = "MERGE INTO target_table a"
        result = apply_merge_layout(sql)
        assert "MERGE INTO\n\ttarget_table a" in result

    def test_when_matched_then_no_indent(self):
        sql = "WHEN MATCHED THEN"
        result = apply_merge_layout(sql)
        assert result.strip() == "WHEN MATCHED THEN"

    def test_when_not_matched_then_no_indent(self):
        sql = "WHEN NOT MATCHED THEN"
        result = apply_merge_layout(sql)
        assert result.strip() == "WHEN NOT MATCHED THEN"

    def test_update_set_gets_indented(self):
        sql = "UPDATE SET\n\ta.col = b.col"
        result = apply_merge_layout(sql)
        assert "\tUPDATE SET" in result

    def test_insert_inside_when_gets_indented(self):
        sql = "INSERT (id, name) VALUES (b.id, b.name)"
        result = apply_merge_layout(sql)
        assert "\tINSERT" in result

    def test_full_merge_structure(self):
        sql = textwrap.dedent("""\
            MERGE INTO target_table a
            USING (
            SELECT col1, col2
            FROM source_table b
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
            UPDATE SET a.col1 = b.col1
            WHERE 1 = 1
            WHEN NOT MATCHED THEN
            INSERT (col1) VALUES (b.col1)
        """)
        result = apply_merge_layout(sql)

        assert "MERGE INTO\n\ttarget_table a" in result
        assert "WHEN MATCHED THEN" in result
        assert "WHEN NOT MATCHED THEN" in result
        assert "\tUPDATE SET" in result
        assert "\tINSERT" in result


class TestMergeIntegration:
    """Integration tests: pipeline completo para MERGE."""

    def test_merge_matched_and_not_matched(self):
        sql = """
            MERGE INTO target_table a
            USING (
                SELECT col1, col2 FROM source_table b
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
                UPDATE SET a.col1 = b.col1
                WHERE 1 = 1
            WHEN NOT MATCHED THEN
                INSERT (col1) VALUES (b.col1)
        """
        result = fmt(sql)
        assert "MERGE INTO\n\ttarget_table" in result
        assert "WHEN MATCHED THEN" in result
        assert "WHEN NOT MATCHED THEN" in result
        assert "\tUPDATE SET" in result
        assert "\tINSERT" in result
        assert result.strip().endswith(";")

    def test_merge_update_only_with_where(self):
        sql = """
            MERGE INTO employees a
            USING (
                SELECT id, salary FROM updates b
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
                UPDATE SET a.salary = b.salary
                WHERE 1 = 1
        """
        result = fmt(sql)
        assert "MERGE INTO\n\temployees" in result
        assert "WHEN MATCHED THEN" in result
        assert "\tUPDATE SET" in result
        assert result.strip().endswith(";")

    def test_merge_using_subquery_has_from_layout(self):
        """FROM em linha própria dentro do USING segue o layout FROM."""
        sql = """
            MERGE INTO orders a
            USING (
                SELECT id, total
                FROM staging_orders b
                WHERE b.status = 'NEW'
            ) b ON (a.id = b.id)
            WHEN MATCHED THEN
                UPDATE SET a.total = b.total
                WHERE 1 = 1
        """
        result = fmt(sql)
        # apply_from_join_layout preserva a indentação original do USING,
        # então staging_orders fica numa linha própria após FROM (com indent)
        lines = result.splitlines()
        from_idx = next(i for i, l in enumerate(lines) if l.strip() == "FROM")
        assert "staging_orders" in lines[from_idx + 1]

    def test_merge_always_ends_with_semicolon(self):
        sql = """
            MERGE INTO t a
            USING (SELECT id FROM s b) b ON (a.id = b.id)
            WHEN MATCHED THEN UPDATE SET a.x = b.x WHERE 1 = 1
        """
        result = fmt(sql)
        assert result.strip().endswith(";")


class TestSemicolonHandling:
    def test_trailing_semicolon_normalized(self):
        result = fmt("SELECT 1;")
        assert result.count(";") == 1
        assert result.strip().endswith(";")

    def test_multiple_trailing_semicolons_normalized(self):
        result = fmt("SELECT 1;;;")
        assert result.count(";") == 1

    def test_output_always_ends_with_semicolon(self):
        result = fmt("SELECT id FROM users")
        assert result.strip().endswith(";")


class TestSubquery:
    def test_subquery_in_where(self):
        sql = "SELECT id FROM users WHERE id IN (SELECT user_id FROM orders WHERE total > 100)"
        result = fmt(sql)
        assert "SELECT" in result
        assert "FROM\n\tusers" in result
        assert result.strip().endswith(";")

    def test_subquery_in_from(self):
        sql = "SELECT sub.id FROM (SELECT id FROM users WHERE active = 1) AS sub"
        result = fmt(sql)
        assert "SELECT" in result
        assert result.strip().endswith(";")


class TestFixIsNotNull:
    def test_simple_column(self):
        assert fix_is_not_null("NOT col IS NULL") == "col IS NOT NULL"

    def test_qualified_column(self):
        assert fix_is_not_null("NOT t.col IS NULL") == "t.col IS NOT NULL"

    def test_quoted_column_with_space(self):
        assert fix_is_not_null('NOT nfr."Nº Order" IS NULL') == 'nfr."Nº Order" IS NOT NULL'

    def test_multiple_occurrences(self):
        result = fix_is_not_null("NOT a IS NULL AND NOT b IS NULL")
        assert result == "a IS NOT NULL AND b IS NOT NULL"

    def test_no_change_when_already_correct(self):
        sql = "col IS NOT NULL"
        assert fix_is_not_null(sql) == sql

    def test_full_pipeline_preserves_is_not_null(self):
        sql = 'SELECT * FROM t WHERE t.col IS NOT NULL'
        result = fmt(sql)
        assert "IS NOT NULL" in result
        assert "NOT t.col IS NULL" not in result


class TestAndOrLayout:
    def test_comment_line_with_and_not_split(self):
        sql = "WHERE\n  a = 1\n  -- AND b = 2 -- comentario inline"
        result = apply_and_or_layout(sql)
        assert result == sql

    def test_comment_line_with_or_not_split(self):
        sql = "WHERE\n  a = 1\n  -- OR b = 2"
        result = apply_and_or_layout(sql)
        assert result == sql

    def test_or_inside_word_not_split(self):
        # "POR" contém "OR" mas não é um operador
        sql = "  AND col = 'FILTRAR POR UM LOTE'"
        result = apply_and_or_layout(sql)
        assert "OR UM LOTE" not in result.split("\n")[-1] or result == sql

    def test_full_pipeline_commented_and_preserved(self):
        sql = """
            SELECT a FROM t
            WHERE x = 1
            --  AND y = 2 -- FILTRAR POR LOTE
            --  AND z = 3
        """
        result = fmt(sql)
        lines = result.split("\n")
        # nenhuma linha deve começar com OR isolado (seria produto de split incorreto)
        assert not any(line.strip().startswith("OR ") or line.strip() == "OR" for line in lines)
        assert "-- AND z = 3" in result

    def test_full_pipeline_por_not_split(self):
        sql = "SELECT * FROM t WHERE col = 'FILTRAR POR UM LOTE' AND other = 1"
        result = fmt(sql)
        lines = result.split("\n")
        # "FILTRAR POR UM LOTE" deve estar intacto numa única linha
        assert any("FILTRAR POR UM LOTE" in line for line in lines)
        # OR não deve aparecer como início de linha separada
        assert not any(line.strip().startswith("OR ") or line.strip() == "OR" for line in lines)
