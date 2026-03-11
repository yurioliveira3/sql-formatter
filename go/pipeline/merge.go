package pipeline

import (
	"regexp"
	"strings"
)

var (
	mergeIntoRe    = regexp.MustCompile(`(?i)^MERGE\s+INTO\s+(.+)$`)
	whenMatchedRe  = regexp.MustCompile(`(?i)^WHEN\s+(NOT\s+)?MATCHED\s+THEN`)
	updateSetRe    = regexp.MustCompile(`(?i)^UPDATE\s+SET\b`)
	insertDeleteRe = regexp.MustCompile(`(?i)^(INSERT|DELETE)\b`)
	whereRe        = regexp.MustCompile(`(?i)^WHERE\b`)
)

// ApplyMergeLayout aplica o layout padrão para instruções MERGE.
func ApplyMergeLayout(sql string) string {
	sql = normalizeNewlines(sql)

	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		stripped := strings.TrimSpace(line)

		if stripped == "" {
			out = append(out, "")
			continue
		}

		// MERGE INTO <table> [alias] → MERGE INTO\n\t<table> [alias]
		if m := mergeIntoRe.FindStringSubmatch(stripped); m != nil {
			out = append(out, "MERGE INTO")
			out = append(out, "\t"+strings.TrimSpace(m[1]))
			continue
		}

		// WHEN [NOT] MATCHED THEN → sem indentação
		if whenMatchedRe.MatchString(stripped) {
			out = append(out, stripped)
			continue
		}

		// UPDATE SET → \tUPDATE SET
		if updateSetRe.MatchString(stripped) {
			out = append(out, "\tUPDATE SET")
			continue
		}

		// INSERT / DELETE → \t<keyword> ...
		if insertDeleteRe.MatchString(stripped) {
			out = append(out, "\t"+stripped)
			continue
		}

		// WHERE → \tWHERE
		if whereRe.MatchString(stripped) {
			out = append(out, "\tWHERE")
			continue
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
