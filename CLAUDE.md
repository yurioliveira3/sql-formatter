# sql-formatter — Claude context

## Project purpose

External SQL formatter for DBeaver. Reads SQL from stdin, writes formatted SQL to stdout.
Two independent implementations — Python (current) and Go (in progress).

## Directory structure

```
sql-formatter/
  python/                    # implementacao Python + sqlglot
    format_sql_ansi.py       # pipeline principal
    test_format_sql.py       # pytest suite
    dist/                    # gerado por PyInstaller (nao commitado)
    build/                   # gerado por PyInstaller (nao commitado)
  go/                        # implementacao Go (em desenvolvimento)
    main.go
    go.mod
    pipeline/
  format_sql_ansi.bat        # entry point DBeaver → Python exe
  format_sql_ansi_go.bat     # entry point DBeaver → Go exe (futuro)
  README.md
  CLAUDE.md
  go/
    REWRITE_GO.md            # plano de reescrita Go
  .gitignore
```

## Python implementation

Entry point: `format_sql_ansi.bat` → `python\dist\format_sql_ansi\format_sql_ansi.exe`

### Pipeline in `main()`:
1. `strip_trailing_semicolons` — normalize input
2. Branch on `is_merge()`:
   - **MERGE path**: `apply_merge_layout` → `apply_from_join_layout`
   - **Normal path**: `preserve_block_comments` → `try_sqlglot` → `strip_trailing_semicolons` → `merge_filter_clauses` → `expand_block_comments` → `convert_block_to_line_comments` → `restore_block_comments` → `apply_from_join_layout` → `apply_limit_layout`
3. `finalize` — adds single `;\n` at end

### Comment handling

sqlglot converts all `--` comments to `/* */`. The pipeline restores them:

1. **`preserve_block_comments`** — antes do sqlglot, substitui `/* */` originais por placeholders `/*__KEEP_n__*/`
2. **`expand_block_comments`** — após sqlglot, expande `/* */` (que vieram de `--`) em linhas próprias:
   - Padrão A: linha inteira de blocos → uma `--` por linha
   - Padrão B: `AND/OR /* comment */ sql_real` → comentários viram `--` antes do AND
   - Padrão C: `code /* c1 */ /* c2 */` (2+ blocos no final) → código + cada `--` em linha própria
3. **`convert_block_to_line_comments`** — converte `/* */` restantes para `--` (ignora placeholders)
4. **`restore_block_comments`** — restaura os `/* */` originais do usuário

**Limitação conhecida:** sqlglot move comentários `--` que precedem colunas do SELECT para depois da última coluna real. A ordem original não é restaurável sem remover os comentários antes do sqlglot.

### Output conventions

- Keywords uppercase (handled by sqlglot)
- Table/value after FROM, JOIN, LIMIT moves to next line with a `\t`
- `ON` clause stays on the same line as the table (sqlglot splits it; `apply_from_join_layout` reacoplates with paren-counting)
- `ON (multi-line condition)` fully merged onto the table line
- `FILTER(WHERE ...)` stays on one line (`merge_filter_clauses` uses paren-counting)
- `--` comments preserved; `/* */` original block comments preserved
- Single `;` on its own line at the end

### Build (Python)

Built with PyInstaller `--onedir --console`. Do NOT use `--onefile` (causes per-execution extraction delay).

```bat
cd python
py -3 -m PyInstaller --onedir --name format_sql_ansi --console -y format_sql_ansi.py
```
The `-y` flag overwrites `python\dist\format_sql_ansi\` without prompting (required on rebuilds).

### Testing (Python)

```bat
cd python
py -3 -m pytest test_format_sql.py -v
```

Tests use `subprocess` to call the script via `sys.executable`, so they test the full pipeline.
Always run the test suite after modifying `python\format_sql_ansi.py`.

## Go implementation

See `go/REWRITE_GO.md` for the full rewrite plan (9 phases).
Entry point (futuro): `format_sql_ansi_go.bat` → `go\dist\format_sql_ansi_go.exe`

### Build (Go)

```bat
cd go
go build -o dist\format_sql_ansi_go.exe .
```

### Testing (Go)

```bat
cd go
go test ./...
```

## Environment

- Windows 11, Python 3.13 (Windows launcher: `py.exe -3`)
- Working directory from WSL: `/mnt/c/projects/sql-formatter`
- Working directory from Windows: `C:\projects\sql-formatter`
- Python dependency: `sqlglot` (installed in Windows Python)
- Go dependency: none (stdlib only, goal)

## Known limitations (Python)

- MERGE uses a custom regex layout — sqlglot does not support ANSI MERGE reliably
- Multi-statement scripts are not a supported use case
- `--` comments that precede SELECT columns are moved by sqlglot to after the last real column (order not recoverable)
