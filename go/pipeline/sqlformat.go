package pipeline

import (
	"strings"
	"unicode"
)

// sqlKeywords mapeia a forma lowercase de cada keyword SQL para sua forma uppercase.
var sqlKeywords = map[string]string{
	"select": "SELECT", "from": "FROM", "where": "WHERE",
	"join": "JOIN", "left": "LEFT", "right": "RIGHT", "inner": "INNER",
	"outer": "OUTER", "full": "FULL", "cross": "CROSS", "natural": "NATURAL",
	"on": "ON", "and": "AND", "or": "OR", "not": "NOT",
	"group": "GROUP", "by": "BY", "order": "ORDER", "having": "HAVING",
	"limit": "LIMIT", "offset": "OFFSET",
	"insert": "INSERT", "into": "INTO", "update": "UPDATE", "set": "SET",
	"delete": "DELETE", "values": "VALUES", "with": "WITH",
	"as": "AS", "case": "CASE", "when": "WHEN", "then": "THEN",
	"else": "ELSE", "end": "END",
	"distinct": "DISTINCT", "union": "UNION", "all": "ALL",
	"except": "EXCEPT", "intersect": "INTERSECT",
	"exists": "EXISTS", "in": "IN", "is": "IS", "null": "NULL",
	"like": "LIKE", "between": "BETWEEN",
	"merge": "MERGE", "using": "USING", "matched": "MATCHED",
	"returning": "RETURNING", "over": "OVER", "partition": "PARTITION",
	"rows": "ROWS", "range": "RANGE", "unbounded": "UNBOUNDED",
	"preceding": "PRECEDING", "following": "FOLLOWING", "current": "CURRENT",
	"row": "ROW", "filter": "FILTER",
	"true": "TRUE", "false": "FALSE",
	"asc": "ASC", "desc": "DESC", "nulls": "NULLS", "first": "FIRST", "last": "LAST",
	"count": "COUNT", "sum": "SUM", "avg": "AVG", "min": "MIN", "max": "MAX",
}

// clauseStarters define keywords que iniciam uma nova linha em profundidade 0.
// Para INNER JOIN, LEFT JOIN etc., o qualificador é quem inicia a linha.
var clauseStarters = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true, "WITH": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
	"JOIN": true,
	"LEFT": true, "RIGHT": true, "INNER": true,
	"FULL": true, "CROSS": true, "NATURAL": true,
	"GROUP": true, "ORDER": true,
}

// joinQualifiers: palavras que, quando precedem JOIN, impedem a quebra de linha.
var joinQualifiers = map[string]bool{
	"LEFT": true, "RIGHT": true, "INNER": true,
	"FULL": true, "CROSS": true, "NATURAL": true, "OUTER": true,
}

// ── Tokenizador ───────────────────────────────────────────────────────────────

type tokKind int

const (
	tkWord         tokKind = iota // palavra/keyword/identificador
	tkString                      // 'literal' ou "literal"
	tkLineComment                 // -- comentário (sem o \n final)
	tkBlockComment                // /* comentário */
	tkNewline                     // \n
	tkSpace                       // espaço/tab (não-newline)
	tkPunct                       // ( ) , . ; etc.
	tkOp                          // operadores e outros caracteres
)

type sqlTok struct {
	kind tokKind
	text string
}

func isWordStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isWordCont(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '#' || r == '@' || r == '$'
}

// tokenize quebra o SQL numa sequência de tokens, preservando strings e comentários.
func tokenize(sql string) []sqlTok {
	rs := []rune(sql)
	n := len(rs)
	out := make([]sqlTok, 0, n/4+8)
	i := 0

	for i < n {
		r := rs[i]

		// Comentário de linha: -- até o fim da linha
		if r == '-' && i+1 < n && rs[i+1] == '-' {
			j := i
			for j < n && rs[j] != '\n' {
				j++
			}
			out = append(out, sqlTok{tkLineComment, string(rs[i:j])})
			i = j
			continue
		}

		// Comentário de bloco: /* ... */
		if r == '/' && i+1 < n && rs[i+1] == '*' {
			j := i + 2
			for j+1 < n && !(rs[j] == '*' && rs[j+1] == '/') {
				j++
			}
			if j+1 < n {
				j += 2 // consome */
			}
			out = append(out, sqlTok{tkBlockComment, string(rs[i:j])})
			i = j
			continue
		}

		// String literal: 'texto' ou "texto"
		if r == '\'' || r == '"' {
			q := r
			j := i + 1
			for j < n {
				if rs[j] == '\\' && j+1 < n { // escape: \' ou \"
					j += 2
					continue
				}
				if rs[j] == q {
					j++ // inclui a aspa de fechamento
					break
				}
				j++
			}
			out = append(out, sqlTok{tkString, string(rs[i:j])})
			i = j
			continue
		}

		// Newline
		if r == '\n' {
			out = append(out, sqlTok{tkNewline, "\n"})
			i++
			continue
		}

		// Espaço/tab (não-newline)
		if unicode.IsSpace(r) {
			j := i
			for j < n && unicode.IsSpace(rs[j]) && rs[j] != '\n' {
				j++
			}
			out = append(out, sqlTok{tkSpace, " "})
			i = j
			continue
		}

		// Palavra (identificador ou keyword)
		if isWordStart(r) {
			j := i
			for j < n && isWordCont(rs[j]) {
				j++
			}
			out = append(out, sqlTok{tkWord, string(rs[i:j])})
			i = j
			continue
		}

		// Número (tratado como word para espaçamento)
		if unicode.IsDigit(r) {
			j := i
			for j < n && (unicode.IsDigit(rs[j]) || rs[j] == '.' || rs[j] == 'e' || rs[j] == 'E') {
				j++
			}
			out = append(out, sqlTok{tkWord, string(rs[i:j])})
			i = j
			continue
		}

		// Pontuação estrutural
		switch r {
		case '(', ')', ',', '.', ';', '[', ']':
			out = append(out, sqlTok{tkPunct, string(r)})
		default:
			// Operadores multi-caractere: >=, <=, <>, !=, ||, ::
			if i+1 < n {
				two := string(rs[i : i+2])
				switch two {
				case ">=", "<=", "<>", "!=", "||", "::", "=>":
					out = append(out, sqlTok{tkOp, two})
					i += 2
					continue
				}
			}
			out = append(out, sqlTok{tkOp, string(r)})
		}
		i++
	}

	return out
}

// ── FormatSQL ─────────────────────────────────────────────────────────────────

// FormatSQL faz uppercase das keywords SQL e estrutura o SQL com cada cláusula
// principal em sua própria linha. Comentários -- são preservados como estão.
func FormatSQL(sql string) string {
	tokens := tokenize(sql)

	var b strings.Builder
	b.Grow(len(sql) + len(sql)/4)

	depth := 0    // profundidade de parênteses
	wantSpace := false  // precisamos de espaço antes do próximo token?
	atLineStart := true // estamos no início de uma linha?
	prevWord := ""      // última palavra/keyword escrita (para detectar INNER JOIN etc.)

	// writeWithSpace escreve text precedido de espaço se necessário.
	writeWithSpace := func(text string) {
		if wantSpace && !atLineStart {
			b.WriteByte(' ')
		}
		b.WriteString(text)
		wantSpace = true
		atLineStart = false
	}

	for _, t := range tokens {
		switch t.kind {

		case tkWord:
			upper := strings.ToUpper(t.text)
			word := t.text
			if kw, ok := sqlKeywords[strings.ToLower(t.text)]; ok {
				word = kw
				upper = kw
			}

			// Decide se este keyword inicia uma nova linha.
			// Regra especial: JOIN após qualificador (INNER, LEFT...) fica na mesma linha.
			shouldBreak := clauseStarters[upper] && depth == 0 && !atLineStart
			if upper == "JOIN" && joinQualifiers[prevWord] {
				shouldBreak = false
			}

			if shouldBreak {
				b.WriteByte('\n')
				atLineStart = true
				wantSpace = false
			}
			writeWithSpace(word)
			prevWord = upper

		case tkString, tkBlockComment:
			writeWithSpace(t.text)
			prevWord = ""

		case tkLineComment:
			// Comentário de linha: escreve na mesma linha, depois força newline
			writeWithSpace(t.text)
			b.WriteByte('\n')
			wantSpace = false
			atLineStart = true
			prevWord = ""

		case tkNewline:
			// Preserva newlines do SQL original (ex: input já formatado)
			if !atLineStart {
				b.WriteByte('\n')
				wantSpace = false
				atLineStart = true
			}
			prevWord = ""

		case tkSpace:
			// Não escrevemos espaço agora — apenas sinalizamos para o próximo token
			wantSpace = true

		case tkPunct:
			switch t.text {
			case "(":
				depth++
				writeWithSpace("(")
				wantSpace = false // sem espaço logo após (
				prevWord = ""
			case ")":
				depth--
				b.WriteByte(')') // sem espaço antes de )
				wantSpace = true
				atLineStart = false
				prevWord = ""
			case ",":
				b.WriteByte(',') // sem espaço antes de ,
				wantSpace = true // espaço após ,
				atLineStart = false
				prevWord = ""
			case ".":
				b.WriteByte('.') // sem espaço antes de .
				wantSpace = false // sem espaço após .
				atLineStart = false
				// prevWord mantém o que era (ex: nome da tabela)
			default:
				writeWithSpace(t.text)
				prevWord = ""
			}

		case tkOp:
			writeWithSpace(t.text)
			prevWord = ""
		}
	}

	result := b.String()

	// Remove linhas em branco duplas que possam ter surgido
	for strings.Contains(result, "\n\n") {
		result = strings.ReplaceAll(result, "\n\n", "\n")
	}

	return strings.TrimSpace(result)
}
