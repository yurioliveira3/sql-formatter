package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

// ── Regex de comentários ──────────────────────────────────────────────────────

var (
	// Qualquer /* ... */ incluindo multilinha
	blockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

	// /* ... */ em uma única linha (sem newline dentro)
	inlineBlockRe = regexp.MustCompile(`/\*([^\n]*?)\*/`)

	// Detecta placeholders __KEEP_n__ dentro do conteúdo de um /* */
	keepRe = regexp.MustCompile(`__KEEP_\d+__`)

	// Linha inteira composta só de blocos /* */ (com possível whitespace)
	pureBlocksLineRe = regexp.MustCompile(`^\s*(?:/\*[^\n]*?\*/\s*)+$`)

	// Padrão B: (indent)(AND|OR) (bloco(s)) (SQL real)
	andOrBlocksRe = regexp.MustCompile(
		`(?i)^(\s*)(AND|OR)\s+(/\*[^\n]*?\*/(?:\s*/\*[^\n]*?\*/)*)\s+(\S.*)$`,
	)
)

// ── PreserveBlockComments ─────────────────────────────────────────────────────

// PreserveBlockComments substitui /* */ originais do usuário por placeholders
// antes do FormatSQL, para que não sejam alterados durante a formatação.
// Retorna (sql_modificado, mapa placeholder→comentário_original).
func PreserveBlockComments(sql string) (string, map[string]string) {
	placeholders := map[string]string{}
	counter := 0

	modified := blockCommentRe.ReplaceAllStringFunc(sql, func(match string) string {
		key := fmt.Sprintf("/*__KEEP_%d__*/", counter)
		placeholders[key] = match
		counter++
		return key
	})

	return modified, placeholders
}

// ── RestoreBlockComments ──────────────────────────────────────────────────────

// RestoreBlockComments restaura os /* */ originais a partir dos placeholders.
// Usa regex tolerante a espaços extras ao redor do conteúdo do placeholder.
func RestoreBlockComments(sql string, placeholders map[string]string) string {
	for key, original := range placeholders {
		content := key[2 : len(key)-2] // remove /* e */
		pattern := regexp.MustCompile(`/\*\s*` + regexp.QuoteMeta(content) + `\s*\*/`)
		sql = pattern.ReplaceAllString(sql, original)
	}
	return sql
}

// ── ExpandBlockComments ───────────────────────────────────────────────────────

// ExpandBlockComments separa comentários /* */ inline em linhas próprias,
// convertendo-os para estilo --.
//
// Padrão A — linha inteira são blocos /* */:
//
//	/* col1 */ /* col2 */  →  -- col1 / -- col2
//
// Padrão B — AND/OR + bloco(s) + SQL real:
//
//	AND /* cond1 */ expr  →  -- cond1 / AND expr
//
// Padrão C — código seguido de 2+ blocos no final da linha:
//
//	lin.* /* c1 */ /* c2 */  →  lin.* / -- c1 / -- c2
//
// Placeholders __KEEP__ são sempre preservados intactos.
func ExpandBlockComments(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		indent := leadingWhitespace(line)

		// Linhas com placeholders __KEEP__ → não aplicar padrões A/B/C
		if keepRe.MatchString(line) {
			// Fallback: converte apenas /* */ que não são placeholders
			out = append(out, convertNonKeepBlocks(line))
			continue
		}

		// Padrão B: AND/OR seguido de bloco(s) de comentário e SQL real
		if m := andOrBlocksRe.FindStringSubmatch(line); m != nil {
			preIndent, keyword, commentsStr, actual := m[1], m[2], m[3], m[4]
			for _, cm := range inlineBlockRe.FindAllStringSubmatch(commentsStr, -1) {
				out = append(out, preIndent+"-- "+strings.TrimSpace(cm[1]))
			}
			out = append(out, preIndent+keyword+" "+actual)
			continue
		}

		// Padrão A: linha inteira composta por blocos de comentário
		if pureBlocksLineRe.MatchString(line) {
			for _, cm := range inlineBlockRe.FindAllStringSubmatch(line, -1) {
				out = append(out, indent+"-- "+strings.TrimSpace(cm[1]))
			}
			continue
		}

		// Padrão C: código seguido de 2+ blocos no final da linha
		trailing := inlineBlockRe.FindAllStringSubmatchIndex(line, -1)
		if len(trailing) >= 2 {
			lastEnd := trailing[len(trailing)-1][1]
			if lastEnd >= len(strings.TrimRight(line, " \t")) {
				firstStart := trailing[0][0]
				codePart := strings.TrimRight(line[:firstStart], " \t")
				if strings.TrimSpace(codePart) != "" {
					out = append(out, codePart)
					for _, cm := range inlineBlockRe.FindAllStringSubmatch(line, -1) {
						out = append(out, indent+"-- "+strings.TrimSpace(cm[1]))
					}
					continue
				}
			}
		}

		// Fallback: converte blocos /* */ restantes para --
		out = append(out, convertNonKeepBlocks(line))
	}

	return strings.Join(out, "\n")
}

// ── ConvertBlockToLineComments ────────────────────────────────────────────────

// ConvertBlockToLineComments converte comentários /* */ para estilo --,
// preservando placeholders __KEEP__ intactos.
func ConvertBlockToLineComments(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, convertNonKeepBlocks(line))
	}
	return strings.Join(out, "\n")
}

// ── helper interno ────────────────────────────────────────────────────────────

// convertNonKeepBlocks converte /* */ em -- na linha, ignorando placeholders __KEEP__.
func convertNonKeepBlocks(line string) string {
	return inlineBlockRe.ReplaceAllStringFunc(line, func(match string) string {
		// Extrai o conteúdo entre /* e */
		inner := inlineBlockRe.FindStringSubmatch(match)[1]
		// Se for um placeholder __KEEP__, não converte
		if keepRe.MatchString(inner) {
			return match
		}
		return "-- " + strings.TrimSpace(inner)
	})
}

