package pipeline_test

import (
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

func TestApplyMergeLayout(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "merge into splits table to next line",
			input: "MERGE INTO target_table a",
			want:  "MERGE INTO\n\ttarget_table a",
		},
		{
			name:  "when matched then no indent",
			input: "WHEN MATCHED THEN",
			want:  "WHEN MATCHED THEN",
		},
		{
			name:  "when not matched then no indent",
			input: "WHEN NOT MATCHED THEN",
			want:  "WHEN NOT MATCHED THEN",
		},
		{
			name:  "update set gets indented",
			input: "UPDATE SET\n\ta.col = b.col",
			want:  "\tUPDATE SET\n\ta.col = b.col",
		},
		{
			name:  "insert inside when gets indented",
			input: "INSERT (id, name) VALUES (b.id, b.name)",
			want:  "\tINSERT (id, name) VALUES (b.id, b.name)",
		},
		{
			name:  "where gets indented",
			input: "WHERE 1 = 1",
			want:  "\tWHERE",
		},
		{
			name: "full merge structure",
			input: "MERGE INTO target_table a\n" +
				"USING (\n" +
				"SELECT col1, col2\n" +
				"FROM source_table b\n" +
				") b ON (a.id = b.id)\n" +
				"WHEN MATCHED THEN\n" +
				"UPDATE SET a.col1 = b.col1\n" +
				"WHERE 1 = 1\n" +
				"WHEN NOT MATCHED THEN\n" +
				"INSERT (col1) VALUES (b.col1)",
			want: "MERGE INTO\n\ttarget_table a\n" +
				"USING (\n" +
				"SELECT col1, col2\n" +
				"FROM source_table b\n" +
				") b ON (a.id = b.id)\n" +
				"WHEN MATCHED THEN\n" +
				"\tUPDATE SET\n" +
				"\tWHERE\n" +
				"WHEN NOT MATCHED THEN\n" +
				"\tINSERT (col1) VALUES (b.col1)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.ApplyMergeLayout(tc.input)
			if got != tc.want {
				t.Errorf("\ngot:\n%s\n\nwant:\n%s", got, tc.want)
			}
		})
	}
}
