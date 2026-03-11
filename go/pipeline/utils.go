package pipeline

import (
	"regexp"
	"strings"
)

var (
	trailingSemicolonsRe = regexp.MustCompile(`(?s)[ \t\r\n]*;+\s*\z`)
	mergeRe              = regexp.MustCompile(`(?i)\s*MERGE\b`)

	orderByRe = regexp.MustCompile(`(?im)^\s*ORDER\s+BY\s+([^\n]+?)\s*$`)
	groupByRe = regexp.MustCompile(`(?im)^\s*GROUP\s+BY\s+([^\n]+?)\s*$`)
	limitRe   = regexp.MustCompile(`(?im)^\s*LIMIT\s+([^\s;]+)\s*$`)
)

// normalizeNewlines converte CRLF e CR para LF.
func normalizeNewlines(sql string) string {
	sql = strings.ReplaceAll(sql, "\r\n", "\n")
	return strings.ReplaceAll(sql, "\r", "\n")
}

// leadingWhitespace retorna a sequência de espaços/tabs no início da linha.
func leadingWhitespace(line string) string {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return line[:i]
		}
	}
	return line
}

func StripTrailingSemicolons(sql string) string {
	return trailingSemicolonsRe.ReplaceAllString(sql, "")
}

func Finalize(sql string) string {
	sql = strings.ReplaceAll(sql, "\r\n", "\n")
	sql = strings.ReplaceAll(sql, "\r", "\n")
	sql = strings.TrimRight(StripTrailingSemicolons(sql), " \t\n")
	return sql + "\n;\n"
}

func IsMerge(sql string) bool {
	return mergeRe.MatchString(sql)
}

// ApplyOrderByLayout formata ORDER BY, GROUP BY e LIMIT colocando o conteúdo
// na linha seguinte com 3 espaços de indentação.
func ApplyOrderByLayout(sql string) string {
	sql = orderByRe.ReplaceAllString(sql, "ORDER BY\n   $1")
	sql = groupByRe.ReplaceAllString(sql, "GROUP BY\n   $1")
	sql = limitRe.ReplaceAllString(sql, "LIMIT\n   $1")
	return sql
}
