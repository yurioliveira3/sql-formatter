# sql-formatter

SQL formatter for ANSI/generic SQL, designed as an external formatter for DBeaver. Reads from stdin, writes to stdout.

Two independent implementations — **Python** (uses sqlglot) and **Go** (stdlib only, no dependencies).

## Output style

```sql
SELECT
   COUNT(DISTINCT o.id) FILTER(WHERE li.id IS NULL) AS orders_sem_line_items,
   name
FROM
   users
INNER JOIN
   orders ON users.id = orders.user_id
LEFT JOIN
   items ON items.order_id = orders.id
WHERE
   active = 1
LIMIT
   10
;
```

---

## Go implementation

### Pipeline (`go/pipeline/`)

1. `StripTrailingSemicolons` — normalizes input
2. Branch on `IsMerge()`:
   - **MERGE**: `ApplyMergeLayout` → `ApplyFromJoinLayout`
   - **Normal**: `PreserveBlockComments` → `FormatSQL` → `StripTrailingSemicolons` → `FixIsNotNull` → `RemoveTableAliasAs` → `MergeFilterClauses` → `ExpandBlockComments` → `ConvertBlockToLineComments` → `RestoreBlockComments` → `ApplySelectLayout` → `ApplyAndOrLayout` → `ApplyWhereLayout` → `ApplyFromJoinLayout` → `ApplyOrderByLayout`
3. `Finalize` — adds single `;\n`

### Build

```bash
cd go
go build -o dist/format_sql_ansi_go .        # Mac/Linux
go build -o dist\format_sql_ansi_go.exe .    # Windows

# Cross-compile for Windows from Mac:
GOOS=windows GOARCH=amd64 go build -o dist/format_sql_ansi_go.exe .
```

### Test

```bash
cd go
go test ./...
```

### Run manually

```bash
echo "SELECT id FROM users WHERE active = 1" | go run .
# or via compiled binary:
echo "SELECT id FROM users WHERE active = 1" | ./dist/format_sql_ansi_go
```

### DBeaver integration

| Platform | Entry point |
|---|---|
| Windows | `format_sql_ansi_go.bat` |
| Mac/Linux | `format_sql_ansi_go.sh` |

Configure DBeaver: **Window → Preferences → Editors → External Formatters** → point to the script above (full path).

---

## Python implementation

### Pipeline (`python/format_sql_ansi.py`)

1. `strip_trailing_semicolons`
2. Branch on `is_merge()`:
   - **MERGE**: `apply_merge_layout` → `apply_from_join_layout`
   - **Normal**: `preserve_block_comments` → `sqlglot` → `strip_trailing_semicolons` → `merge_filter_clauses` → `expand_block_comments` → `convert_block_to_line_comments` → `restore_block_comments` → `apply_from_join_layout` → `apply_limit_layout`
3. `finalize`

### Requirements

- Python 3.10+
- `sqlglot`: `py -3 -m pip install sqlglot`
- PyInstaller (to rebuild exe): `py -3 -m pip install pyinstaller`

### Build executable

```bat
cd python
py -3 -m PyInstaller --onedir --name format_sql_ansi --console -y format_sql_ansi.py
```

Use `--onedir` (not `--onefile`) to avoid per-execution extraction overhead.

### Test

```bat
cd python
py -3 -m pytest test_format_sql.py -v
```

### DBeaver integration (Windows only)

Entry point: `format_sql_ansi.bat` → `python\dist\format_sql_ansi\format_sql_ansi.exe`

---

## Files

| File | Description |
|---|---|
| `format_sql_ansi.bat` | DBeaver entry point — Python (Windows) |
| `format_sql_ansi_go.bat` | DBeaver entry point — Go (Windows) |
| `format_sql_ansi_go.sh` | DBeaver entry point — Go (Mac/Linux) |
| `python/format_sql_ansi.py` | Python formatter |
| `python/test_format_sql.py` | Python test suite |
| `go/main.go` | Go entry point |
| `go/pipeline/` | Go pipeline implementation and tests |

## Known limitations (Python)

- MERGE uses a custom regex layout — sqlglot does not support ANSI MERGE reliably.
- `--` comments before SELECT columns are moved by sqlglot to after the last column; original order is not recoverable.
- Multi-statement scripts are not the primary use case.
