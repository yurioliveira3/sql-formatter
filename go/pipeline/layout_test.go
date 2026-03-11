package pipeline_test

import (
	"testing"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

func TestApplyFromJoinLayout(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "from moves table to next line",
			input: "FROM users",
			want:  "FROM\n   users",
		},
		{
			name:  "join moves table to next line",
			input: "JOIN orders ON users.id = orders.user_id",
			want:  "JOIN\n   orders ON users.id = orders.user_id",
		},
		{
			name:  "left join",
			input: "LEFT JOIN orders ON users.id = orders.user_id",
			want:  "LEFT JOIN\n   orders ON users.id = orders.user_id",
		},
		{
			name:  "inner join",
			input: "INNER JOIN orders ON users.id = orders.user_id",
			want:  "INNER JOIN\n   orders ON users.id = orders.user_id",
		},
		{
			name:  "right join",
			input: "RIGHT JOIN dept ON e.dept_id = dept.id",
			want:  "RIGHT JOIN\n   dept ON e.dept_id = dept.id",
		},
		{
			name:  "full outer join",
			input: "FULL OUTER JOIN dept ON e.dept_id = dept.id",
			want:  "FULL OUTER JOIN\n   dept ON e.dept_id = dept.id",
		},
		{
			name:  "from preserves indentation",
			input: "  FROM sub_table",
			want:  "  FROM\n     sub_table",
		},
		{
			name:  "lone from keyword untouched",
			input: "FROM",
			want:  "FROM",
		},
		{
			name:  "loose ON line gets merged back",
			input: "JOIN orders\n  ON users.id = orders.user_id",
			want:  "JOIN\n   orders ON users.id = orders.user_id",
		},
		{
			name:  "ON without leading whitespace gets merged",
			input: "LEFT JOIN items\nON items.order_id = orders.id",
			want:  "LEFT JOIN\n   items ON items.order_id = orders.id",
		},
		{
			name:  "ON with multi-line parens merged",
			input: "JOIN orders\n  ON (\n    users.id = orders.user_id\n  )",
			want:  "JOIN\n   orders ON ( users.id = orders.user_id )",
		},
		{
			name: "multiple joins",
			input: "SELECT u.id\n" +
				"FROM users u\n" +
				"INNER JOIN orders o ON u.id = o.user_id\n" +
				"LEFT JOIN products p ON o.product_id = p.id",
			want: "SELECT u.id\n" +
				"FROM\n   users u\n" +
				"INNER JOIN\n   orders o ON u.id = o.user_id\n" +
				"LEFT JOIN\n   products p ON o.product_id = p.id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipeline.ApplyFromJoinLayout(tc.input)
			if got != tc.want {
				t.Errorf("\ngot:\n%q\n\nwant:\n%q", got, tc.want)
			}
		})
	}
}
