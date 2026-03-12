package pipeline

import (
	"regexp"
	"strings"
)

var (
	// Captura: (indent)(keyword JOIN)(resto)
	joinRe = regexp.MustCompile(
		`(?i)^(\s*)` +
			`((?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN|JOIN)` +
			`\s+(.+?)\s*$`,
	)
	// JOIN sozinho na linha (sem tabela)
	loneJoinRe = regexp.MustCompile(
		`(?i)^(\s*)((?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN|JOIN)\s*$`,
	)

	// Captura: (indent) FROM (resto)
	fromRe = regexp.MustCompile(`(?i)^(\s*)FROM\s+(.+?)\s*$`)

	// FROM sozinho na linha
	loneFromRe = regexp.MustCompile(`(?i)^(\s*)FROM\s*$`)

	// ON solto no início da linha (com ou sem whitespace inicial)
	looseOnRe = regexp.MustCompile(`(?i)^\s*ON\b`)

	onWordRe = regexp.MustCompile(`(?i)\bON\b`)

	// Detecta início de nova cláusula SQL (para saber quando parar o lookahead)
	newClauseRe = regexp.MustCompile(
		`(?i)^\s*(SELECT|FROM|WHERE|HAVING|LIMIT|OFFSET|WITH|UNION|INTERSECT|EXCEPT|` +
			`(?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN|JOIN|GROUP\s+BY|ORDER\s+BY)\b`,
	)
)

// isLineComment retorna true se a linha (após trim) é um comentário --.
func isLineComment(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "--")
}

// mergeOnClauses reacola linhas ON soltas e parênteses multilinhas ao rest.
// Atualiza i conforme consume linhas. Retorna (rest atualizado, i atualizado).
func mergeOnClauses(lines []string, i int, rest string) (string, int) {
	// Reacola linha(s) de ON solta na linha da tabela
	for i+1 < len(lines) && looseOnRe.MatchString(lines[i+1]) {
		i++
		rest = rest + " " + strings.TrimSpace(lines[i])
	}

	// Se o ON abre parênteses, continua reacoplando até fechar
	if loc := onWordRe.FindStringIndex(rest); loc != nil {
		afterOn := rest[loc[0]:]
		depth := strings.Count(afterOn, "(") - strings.Count(afterOn, ")")
		for depth > 0 && i+1 < len(lines) {
			i++
			cont := strings.TrimSpace(lines[i])
			rest = rest + " " + cont
			depth += strings.Count(cont, "(") - strings.Count(cont, ")")
		}
	}

	return rest, i
}

// ApplyFromJoinLayout reformata FROM/JOIN colocando a tabela na linha seguinte.
// Reacola linhas ON soltas na linha da tabela correspondente.
// Suporta FROM/JOIN sozinhos (tabela na próxima linha, possivelmente com comentários no meio).
func ApplyFromJoinLayout(sql string) string {
	sql = normalizeNewlines(sql)

	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// FROM sozinho → look ahead para a tabela
		if m := loneFromRe.FindStringSubmatch(line); m != nil {
			indent := m[1]
			out = append(out, indent+"FROM")
			// Próxima linha não-cláusula é a tabela
			if i+1 < len(lines) && !newClauseRe.MatchString(lines[i+1]) && !isLineComment(lines[i+1]) {
				i++
				out = append(out, indent+"   "+strings.TrimSpace(lines[i]))
			}
			continue
		}

		// FROM <tabela> → FROM\n   <tabela>
		if m := fromRe.FindStringSubmatch(line); m != nil {
			indent, rest := m[1], m[2]
			out = append(out, indent+"FROM")
			out = append(out, indent+"   "+rest)
			continue
		}

		// JOIN sozinho → look ahead past comentários, depois tabela + ON
		if m := loneJoinRe.FindStringSubmatch(line); m != nil {
			indent, joinKw := m[1], strings.TrimSpace(m[2])
			out = append(out, indent+joinKw)
			// Coleta comentários intermediários (indentados junto)
			for i+1 < len(lines) && isLineComment(lines[i+1]) {
				i++
				out = append(out, indent+"   "+strings.TrimSpace(lines[i]))
			}
			// Tabela na próxima linha (se não for nova cláusula)
			if i+1 < len(lines) && !newClauseRe.MatchString(lines[i+1]) {
				i++
				rest := strings.TrimSpace(lines[i])
				rest, i = mergeOnClauses(lines, i, rest)
				out = append(out, indent+"   "+rest)
			}
			continue
		}

		// JOIN <tabela> [ON ...]
		if m := joinRe.FindStringSubmatch(line); m != nil {
			indent, joinKw, rest := m[1], m[2], m[3]
			rest, i = mergeOnClauses(lines, i, rest)
			out = append(out, indent+strings.TrimSpace(joinKw))
			out = append(out, indent+"   "+rest)
			continue
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
