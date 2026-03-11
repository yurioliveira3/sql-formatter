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
	// Captura: (indent) FROM (resto)
	fromRe = regexp.MustCompile(`(?i)^(\s*)FROM\s+(.+?)\s*$`)

	// FROM sozinho na linha
	loneFromRe = regexp.MustCompile(`(?i)^\s*FROM\s*$`)

	// ON solto no início da linha (com ou sem whitespace inicial)
	looseOnRe = regexp.MustCompile(`(?i)^\s*ON\b`)

	onWordRe = regexp.MustCompile(`(?i)\bON\b`)
)

// ApplyFromJoinLayout reformata FROM/JOIN colocando a tabela na linha seguinte.
// Reacola linhas ON soltas na linha da tabela correspondente.
func ApplyFromJoinLayout(sql string) string {
	sql = normalizeNewlines(sql)

	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// FROM sozinho → deixa como está
		if loneFromRe.MatchString(line) {
			trimmed := strings.TrimSpace(line)
			if strings.ToUpper(trimmed) == "FROM" {
				out = append(out, trimmed)
			} else {
				out = append(out, line)
			}
			continue
		}

		// FROM <tabela> → FROM\n\t<tabela>
		if m := fromRe.FindStringSubmatch(line); m != nil {
			indent, rest := m[1], m[2]
			out = append(out, indent+"FROM")
			out = append(out, indent+"   "+rest)
			continue
		}

		// JOIN <tabela> [ON ...]
		if m := joinRe.FindStringSubmatch(line); m != nil {
			indent, joinKw, rest := m[1], m[2], m[3]

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

			out = append(out, indent+strings.TrimSpace(joinKw))
			out = append(out, indent+"   "+rest)
			continue
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
