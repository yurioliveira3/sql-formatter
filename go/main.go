package main

import (
	"io"
	"os"

	"github.com/yurioliveira3/sql-formatter/pipeline"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(1)
	}
	os.Stdout.WriteString(pipeline.Format(string(input)))
}
