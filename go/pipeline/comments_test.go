package pipeline_test

import (
	"strings"
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

// ── PreserveBlockComments ─────────────────────────────────────────────────────

func TestPreserveBlockComments(t *testing.T) {
	t.Run("single block comment replaced by placeholder", func(t *testing.T) {
		sql := "SELECT /* comentário */ id FROM t"
		got, m := pipeline.PreserveBlockComments(sql)

		// O comentário original deve ter sumido do SQL
		if got == sql {
			t.Error("SQL não foi modificado")
		}
		// Deve existir um placeholder no SQL modificado
		if len(m) != 1 {
			t.Errorf("esperava 1 placeholder, got %d", len(m))
		}
		// O placeholder deve referenciar o comentário original
		for k, v := range m {
			if v != "/* comentário */" {
				t.Errorf("placeholder %q mapeia para %q, esperava %q", k, v, "/* comentário */")
			}
			// O placeholder deve estar no SQL
			if !containsStr(got, k) {
				t.Errorf("placeholder %q não encontrado no SQL: %s", k, got)
			}
		}
	})

	t.Run("multiple block comments get unique placeholders", func(t *testing.T) {
		sql := "SELECT /* c1 */ id, /* c2 */ name FROM t"
		got, m := pipeline.PreserveBlockComments(sql)

		if len(m) != 2 {
			t.Errorf("esperava 2 placeholders, got %d: %v", len(m), m)
		}
		_ = got
	})

	t.Run("no block comments returns sql unchanged", func(t *testing.T) {
		sql := "SELECT id FROM t WHERE active = 1"
		got, m := pipeline.PreserveBlockComments(sql)

		if got != sql {
			t.Errorf("SQL não deveria ter mudado: got %q", got)
		}
		if len(m) != 0 {
			t.Errorf("esperava mapa vazio, got %v", m)
		}
	})

	t.Run("line comments are not touched", func(t *testing.T) {
		sql := "SELECT id -- comentário\nFROM t"
		got, m := pipeline.PreserveBlockComments(sql)

		if got != sql {
			t.Errorf("SQL com -- não deveria mudar: got %q", got)
		}
		if len(m) != 0 {
			t.Errorf("esperava mapa vazio, got %v", m)
		}
	})

	t.Run("multiline block comment preserved", func(t *testing.T) {
		sql := "SELECT /* linha1\nlinha2 */ id FROM t"
		_, m := pipeline.PreserveBlockComments(sql)
		if len(m) != 1 {
			t.Errorf("esperava 1 placeholder, got %d", len(m))
		}
		for _, v := range m {
			if v != "/* linha1\nlinha2 */" {
				t.Errorf("conteúdo do placeholder incorreto: %q", v)
			}
		}
	})
}

// ── RestoreBlockComments ──────────────────────────────────────────────────────

func TestRestoreBlockComments(t *testing.T) {
	t.Run("placeholder restaurado para comentário original", func(t *testing.T) {
		original := "SELECT /* meu comentário */ id FROM t"
		modified, m := pipeline.PreserveBlockComments(original)

		restored := pipeline.RestoreBlockComments(modified, m)

		if restored != original {
			t.Errorf("got %q, want %q", restored, original)
		}
	})

	t.Run("multiple placeholders restaurados", func(t *testing.T) {
		original := "SELECT /* c1 */ id, /* c2 */ name FROM t"
		modified, m := pipeline.PreserveBlockComments(original)
		restored := pipeline.RestoreBlockComments(modified, m)

		if restored != original {
			t.Errorf("got %q, want %q", restored, original)
		}
	})

	t.Run("sem placeholders, SQL inalterado", func(t *testing.T) {
		sql := "SELECT id FROM t"
		result := pipeline.RestoreBlockComments(sql, map[string]string{})
		if result != sql {
			t.Errorf("got %q, want %q", result, sql)
		}
	})

	t.Run("restaura mesmo com espaços extras ao redor do conteúdo", func(t *testing.T) {
		m := map[string]string{"/*__KEEP_0__*/": "/* original */"}
		// simula que o FormatSQL adicionou espaço: /*  __KEEP_0__  */
		sql := "SELECT /*  __KEEP_0__  */ id FROM t"
		result := pipeline.RestoreBlockComments(sql, m)
		if !containsStr(result, "/* original */") {
			t.Errorf("comentário não restaurado, got: %q", result)
		}
	})
}

// ── ConvertBlockToLineComments ────────────────────────────────────────────────

func TestConvertBlockToLineComments(t *testing.T) {
	t.Run("block comment convertido para linha", func(t *testing.T) {
		sql := "SELECT /* comentário */ id FROM t"
		got := pipeline.ConvertBlockToLineComments(sql)
		if !containsStr(got, "-- comentário") {
			t.Errorf("esperava '-- comentário', got: %q", got)
		}
	})

	t.Run("placeholder KEEP não é convertido", func(t *testing.T) {
		sql := "SELECT /*__KEEP_0__*/ id FROM t"
		got := pipeline.ConvertBlockToLineComments(sql)
		// placeholder deve sobreviver intacto
		if !containsStr(got, "/*__KEEP_0__*/") {
			t.Errorf("placeholder foi convertido indevidamente: %q", got)
		}
	})

	t.Run("multiple block comments convertidos", func(t *testing.T) {
		sql := "/* c1 */ SELECT /* c2 */ id FROM t"
		got := pipeline.ConvertBlockToLineComments(sql)
		if !containsStr(got, "-- c1") || !containsStr(got, "-- c2") {
			t.Errorf("comentários não convertidos: %q", got)
		}
	})

	t.Run("multiline block comment NÃO é convertido (contém newline)", func(t *testing.T) {
		// ConvertBlockToLineComments só converte /* */ de uma única linha.
		sql := "SELECT /* multi\nlinha */ id FROM t"
		got := pipeline.ConvertBlockToLineComments(sql)
		// o comentário multilinha deve permanecer como /* */
		if !containsStr(got, "/*") {
			t.Errorf("comentário multilinha foi indevidamente convertido: %q", got)
		}
	})
}

// ── ExpandBlockComments ───────────────────────────────────────────────────────

func TestExpandBlockComments(t *testing.T) {
	t.Run("padrão A: linha inteira de blocos vira linhas de --", func(t *testing.T) {
		// Linha com apenas comentários de bloco
		sql := "  /* col1 */ /* col2 */"
		got := pipeline.ExpandBlockComments(sql)
		if !containsStr(got, "-- col1") || !containsStr(got, "-- col2") {
			t.Errorf("padrão A falhou: %q", got)
		}
	})

	t.Run("padrão B: AND/OR + bloco(s) + SQL real", func(t *testing.T) {
		// AND /* cond1 */ expr_real → -- cond1\nAND expr_real
		sql := "  AND /* cond1 */ expr_real"
		got := pipeline.ExpandBlockComments(sql)
		if !containsStr(got, "-- cond1") {
			t.Errorf("padrão B: comentário não extraído: %q", got)
		}
		if !containsStr(got, "AND expr_real") {
			t.Errorf("padrão B: SQL real não preservado: %q", got)
		}
	})

	t.Run("padrão C: código + 2 blocos no final", func(t *testing.T) {
		// code /* c1 */ /* c2 */ → code\n-- c1\n-- c2
		sql := "  lin.* /* col1 */ /* col2 */"
		got := pipeline.ExpandBlockComments(sql)
		if !containsStr(got, "lin.*") || !containsStr(got, "-- col1") || !containsStr(got, "-- col2") {
			t.Errorf("padrão C falhou: %q", got)
		}
	})

	t.Run("placeholder KEEP não é expandido", func(t *testing.T) {
		sql := "SELECT /*__KEEP_0__*/ id FROM t"
		got := pipeline.ExpandBlockComments(sql)
		if !containsStr(got, "/*__KEEP_0__*/") {
			t.Errorf("placeholder foi alterado: %q", got)
		}
	})

	t.Run("linha sem blocos fica inalterada", func(t *testing.T) {
		sql := "SELECT id FROM t"
		got := pipeline.ExpandBlockComments(sql)
		if got != sql {
			t.Errorf("linha sem blocos foi alterada: got %q", got)
		}
	})
}

// ── helper local ──────────────────────────────────────────────────────────────

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
