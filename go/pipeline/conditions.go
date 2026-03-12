package pipeline

import (
	"regexp"
	"strings"
)

// ── Regex ─────────────────────────────────────────────────────────────────────

var (
	// NOT <expr> IS NULL  →  <expr> IS NOT NULL
	notIsNullRe = regexp.MustCompile(`(?i)\bNOT\s+(.+?)\s+IS\s+NULL\b`)

	// FROM/JOIN <tabela> AS <alias>
	fromJoinTableAsRe = regexp.MustCompile(
		`(?i)^(\s*(?:(?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN|JOIN|FROM)\s+\S+)\s+AS\s+(\w+)`,
	)

	// ) AS <alias>  (subquery alias)
	closeParenAsRe = regexp.MustCompile(`(?i)^(\s*\))\s+AS\s+(\w+)`)

	// FILTER( — detecta início de cláusula FILTER
	filterRe = regexp.MustCompile(`(?i)\bFILTER\s*\(`)

	// AND/OR como palavra separada (com espaço depois) — para split
	andOrWordRe = regexp.MustCompile(`(?i)\b(AND|OR)\s`)

	// AND/OR — quick check antes de processar linha
	andOrQuickRe = regexp.MustCompile(`(?i)\b(?:AND|OR)\s`)
)

// ── FixIsNotNull ──────────────────────────────────────────────────────────────

// FixIsNotNull reverte a transformação que converte IS NOT NULL em NOT <expr> IS NULL.
func FixIsNotNull(sql string) string {
	return notIsNullRe.ReplaceAllString(sql, "$1 IS NOT NULL")
}

// ── RemoveTableAliasAs ────────────────────────────────────────────────────────

// RemoveTableAliasAs remove o AS de aliases de tabela em FROM/JOIN.
// Preserva AS em aliases de coluna no SELECT.
func RemoveTableAliasAs(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		// FROM/JOIN <tabela> AS <alias>
		if m := fromJoinTableAsRe.FindStringSubmatchIndex(line); m != nil {
			// m[2],m[3] = grupo 1 (tudo antes do AS alias)
			// m[4],m[5] = grupo 2 (o alias)
			// Reconstrói: grupo1 + " " + alias + resto após o match completo
			g1 := line[m[2]:m[3]]
			alias := line[m[4]:m[5]]
			rest := line[m[1]:]
			line = g1 + " " + alias + rest
		} else if m := closeParenAsRe.FindStringSubmatchIndex(line); m != nil {
			g1 := line[m[2]:m[3]]
			alias := line[m[4]:m[5]]
			rest := line[m[1]:]
			line = g1 + " " + alias + rest
		}
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

// ── MergeFilterClauses ────────────────────────────────────────────────────────

// MergeFilterClauses reúne linhas de FILTER(WHERE ...) que foram quebradas em
// múltiplas linhas. Conta parênteses para saber quando o FILTER( foi fechado.
func MergeFilterClauses(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))
	i := 0

	for i < len(lines) {
		line := lines[i]

		if m := filterRe.FindStringIndex(line); m != nil {
			// Conta parênteses a partir do FILTER(
			after := line[m[0]:]
			depth := strings.Count(after, "(") - strings.Count(after, ")")

			// Continua acoplando linhas até fechar todos os parênteses
			for depth > 0 && i+1 < len(lines) {
				i++
				continuation := strings.TrimSpace(lines[i])
				line = line + " " + continuation
				depth += strings.Count(continuation, "(") - strings.Count(continuation, ")")
			}
		}

		out = append(out, line)
		i++
	}

	return strings.Join(out, "\n")
}

// ── SplitTopLevelAndOr ────────────────────────────────────────────────────────

// SplitTopLevelAndOr divide uma linha nos AND/OR de nível superior
// (fora de parênteses e strings). Retorna as partes; se não houver split,
// retorna slice com a linha original.
func SplitTopLevelAndOr(line string) []string {
	n := len(line)
	depthAt := make([]int, n)
	skipAt := make([]bool, n) // true = dentro de string literal

	depth := 0
	inString := false
	stringChar := rune(0)

	for i, ch := range line {
		if inString {
			skipAt[i] = true
			if ch == stringChar {
				inString = false
			}
		} else if ch == '\'' || ch == '"' {
			skipAt[i] = true
			inString = true
			stringChar = ch
		} else {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			}
		}
		depthAt[i] = depth
	}

	// Encontra todas as posições onde AND/OR está em depth 0 e fora de string
	var splitPoints []int
	for _, m := range andOrWordRe.FindAllStringIndex(line, -1) {
		pos := m[0]
		if pos < n && depthAt[pos] == 0 && !skipAt[pos] {
			splitPoints = append(splitPoints, pos)
		}
	}

	if len(splitPoints) == 0 {
		return []string{line}
	}

	// Divide a linha nos pontos de split
	parts := make([]string, 0, len(splitPoints)+1)
	prev := 0
	for _, pos := range splitPoints {
		parts = append(parts, strings.TrimRight(line[prev:pos], " \t"))
		prev = pos
	}
	parts = append(parts, line[prev:])

	return parts
}

// ── ApplySelectLayout ────────────────────────────────────────────────────────

var (
	selectWithContentRe = regexp.MustCompile(`(?i)^(\s*)SELECT\s+(.+)$`)
	selectAloneRe       = regexp.MustCompile(`(?i)^(\s*)SELECT\s*$`)
)

// splitTopLevelCommas divide uma string nas vírgulas de nível superior
// (fora de parênteses e strings). Usado para separar colunas do SELECT.
func splitTopLevelCommas(s string) []string {
	depth := 0
	inString := false
	stringChar := rune(0)
	prev := 0
	var parts []string

	for i, ch := range s {
		if inString {
			if ch == stringChar {
				inString = false
			}
		} else if ch == '\'' || ch == '"' {
			inString = true
			stringChar = ch
		} else if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		} else if ch == ',' && depth == 0 {
			parts = append(parts, s[prev:i])
			prev = i + 1
		}
	}
	parts = append(parts, s[prev:])
	return parts
}

// ApplySelectLayout coloca cada coluna do SELECT em sua própria linha indentada.
// Coluna única permanece na mesma linha do SELECT.
// Suporta dois padrões: "SELECT col1, col2" e "SELECT\ncol1, col2".
func ApplySelectLayout(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines)*2)
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Padrão 1: SELECT com conteúdo na mesma linha
		if m := selectWithContentRe.FindStringSubmatch(line); m != nil {
			lineIndent := m[1]
			cols := splitTopLevelCommas(strings.TrimSpace(m[2]))
			if len(cols) <= 1 {
				out = append(out, line)
			} else {
				out = append(out, lineIndent+"SELECT")
				condIndent := lineIndent + "   "
				for j, col := range cols {
					col = strings.TrimSpace(col)
					if j < len(cols)-1 {
						out = append(out, condIndent+col+",")
					} else {
						out = append(out, condIndent+col)
					}
				}
			}
			i++
			continue
		}

		// Padrão 2: SELECT sozinho, colunas na próxima linha (uma ou mais)
		if m := selectAloneRe.FindStringSubmatch(line); m != nil && i+1 < len(lines) {
			lineIndent := m[1]
			nextContent := strings.TrimSpace(lines[i+1])
			if nextContent != "" {
				cols := splitTopLevelCommas(nextContent)
				out = append(out, lineIndent+"SELECT")
				condIndent := lineIndent + "   "
				for j, col := range cols {
					col = strings.TrimSpace(col)
					if j < len(cols)-1 {
						out = append(out, condIndent+col+",")
					} else {
						out = append(out, condIndent+col)
					}
				}
				i += 2 // consome a linha SELECT e a linha de colunas
				continue
			}
		}

		out = append(out, line)
		i++
	}

	return strings.Join(out, "\n")
}

// ── ApplyWhereLayout ──────────────────────────────────────────────────────────

var (
	whereHavingContentRe = regexp.MustCompile(`(?i)^(\s*)(WHERE|HAVING)\s+(.+)$`)
	whereHavingAloneRe   = regexp.MustCompile(`(?i)^(\s*)(WHERE|HAVING)\s*$`)
	andOrLineRe          = regexp.MustCompile(`(?i)^(\s*)(AND|OR)\s+(.+)$`)
)

// ApplyWhereLayout coloca a primeira condição WHERE/HAVING e todos os AND/OR
// subsequentes em linhas indentadas abaixo do keyword.
// Suporta WHERE/HAVING na mesma linha ou sozinhos com condição na próxima linha.
func ApplyWhereLayout(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))
	i := 0

	for i < len(lines) {
		line := lines[i]

		var lineIndent, keyword, condition string

		if m := whereHavingContentRe.FindStringSubmatch(line); m != nil {
			// WHERE <condição> na mesma linha
			lineIndent = m[1]
			keyword = strings.ToUpper(m[2])
			condition = strings.TrimSpace(m[3])
		} else if m := whereHavingAloneRe.FindStringSubmatch(line); m != nil && i+1 < len(lines) {
			// WHERE sozinho, condição na próxima linha
			lineIndent = m[1]
			keyword = strings.ToUpper(m[2])
			i++
			condition = strings.TrimSpace(lines[i])
		} else {
			out = append(out, line)
			i++
			continue
		}

		condIndent := lineIndent + "   "
		out = append(out, lineIndent+keyword)
		out = append(out, condIndent+condition)
		i++

		// Coleta AND/OR que seguem no mesmo nível de indentação
		for i < len(lines) {
			am := andOrLineRe.FindStringSubmatch(lines[i])
			if am == nil || am[1] != lineIndent {
				break
			}
			andOr := strings.ToUpper(am[2])
			rest := strings.TrimSpace(am[3])
			out = append(out, condIndent+andOr+" "+rest)
			i++
		}
	}

	return strings.Join(out, "\n")
}

// ── ApplyAndOrLayout ──────────────────────────────────────────────────────────

// ApplyAndOrLayout quebra condições AND/OR de nível superior em linhas próprias.
// Linhas de comentário (--) nunca são quebradas.
func ApplyAndOrLayout(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		// Short-circuit: ignora linhas sem AND/OR
		if !andOrQuickRe.MatchString(line) {
			out = append(out, line)
			continue
		}
		// Linhas de comentário -- nunca são quebradas
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			out = append(out, line)
			continue
		}

		indent := leadingWhitespace(line)
		parts := SplitTopLevelAndOr(line)

		if len(parts) > 1 {
			for _, part := range parts {
				stripped := strings.TrimSpace(part)
				if stripped != "" {
					out = append(out, indent+stripped)
				}
			}
		} else {
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}
