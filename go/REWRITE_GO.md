# Plano de reescrita: sql-formatter em Go

## Contexto

Reescrita do formatter Python+sqlglot+PyInstaller em Go puro.
Objetivo: binario nativo com startup ~10x mais rapido, sem dependencias de runtime, sem PyInstaller.
Todas as regras de negocio do pipeline atual devem ser mantidas.

---

## Desafio principal: substituir o sqlglot

O sqlglot faz tres coisas no pipeline atual:
1. Coloca keywords em maiusculas
2. Emite o SQL estruturado (cada clausula em linha propria, indentacao de subqueries)
3. Converte comentarios `--` para `/* */` (efeito colateral que o pipeline usa)

**Estrategia:** implementar keyword uppercasing com regex + normalizacao de whitespace.
O pipeline customizado ja cuida de 80% do layout. Caso o parser nao cubra algum caso,
aplicar fallback (pass-through), igual ao que o Python faz hoje quando sqlglot falha.

---

## Comparativo de performance esperada

| Etapa | Python atual | Go reescrito |
|---|---|---|
| Startup (PyInstaller `--onedir`) | ~150-300 ms | ~5-15 ms |
| Processamento do pipeline | < 10 ms | < 5 ms |
| **Total** | **~160-310 ms** | **~10-20 ms** |

---

## Fases de implementacao

### Fase 1 — Skeleton do projeto Go

**Entregavel:** binario que le stdin e escreve stdout sem processar.

- `go mod init github.com/yurioliveira3/sql-formatter`
- `main.go` com leitura de `os.Stdin` e escrita em `os.Stdout`
- `go build -o format_sql_ansi.exe .`
- Atualizar `.gitignore` (adicionar `*.exe`)

Arquivos criados: `go.mod`, `main.go`

---

### Fase 2 — Utilitarios e funcoes simples

**Entregavel:** funcoes sem dependencias externas portadas, com testes unitarios.

Funcoes a portar de `format_sql_ansi.py`:
- `strip_trailing_semicolons` — regex simples
- `finalize` — adiciona `;\n`
- `is_merge` — regex match
- `apply_limit_layout` — regex substitution
- `apply_merge_layout` — iteracao linha a linha com regex

Todas as regex pre-compiladas como `var` no nivel de pacote (igual ao Python).

Arquivos: `pipeline/utils.go`, `pipeline/merge.go`, com testes `*_test.go`.

---

### Fase 3 — Layout FROM/JOIN

**Entregavel:** `apply_from_join_layout` portada com contagem de parenteses.

Logica critica:
- `FROM <tabela>` → `FROM\n\t<tabela>`
- `JOIN <tabela>` → `JOIN\n\t<tabela> ON ...`
- Acoplar linhas `ON` soltas (loop com contagem de profundidade de parenteses)
- Preservar indentacao de subqueries

Arquivos: `pipeline/layout.go`, `pipeline/layout_test.go`

---

### Fase 4 — Substituicao do sqlglot (ponto critico)

**Entregavel:** funcao `formatSQL(sql string) string`.

Estrategia concreta:
1. Regex para keyword uppercasing das principais keywords SQL
2. Normalizacao de `\r\n` → `\n`
3. Passar pelo pipeline customizado
4. Validar contra os testes — identificar gaps e ajustar iterativamente

Arquivos: `pipeline/sqlformat.go`, `pipeline/sqlformat_test.go`

---

### Fase 5 — Handling de comentarios

**Entregavel:** ciclo completo de comentarios portado e testado.

Funcoes a portar:
- `preserve_block_comments` — substitui `/* */` originais por placeholders `/*__KEEP_n__*/`
- `expand_block_comments` — 3 padroes:
  - A: linha inteira de blocos → uma `--` por linha
  - B: `AND/OR /* comment */ sql_real` → comentario vira `--` antes do AND
  - C: `code /* c1 */ /* c2 */` (2+ blocos no final) → codigo + cada `--` em linha
- `restore_block_comments` — restaura placeholders

Em Go, os placeholders usam `map[string]string` retornado como segundo valor.

Arquivos: `pipeline/comments.go`, `pipeline/comments_test.go`

---

### Fase 6 — AND/OR layout e FILTER

**Entregavel:** `apply_and_or_layout`, `merge_filter_clauses` e `remove_table_alias_as` portadas.

- `split_top_level_and_or` — iteracao runa por runa com tracking de profundidade e strings
- `apply_and_or_layout` — iteracao linha a linha
- `merge_filter_clauses` — contagem de parenteses para reunir `FILTER(WHERE...)`
- `remove_table_alias_as` — regex para remover `AS` de aliases de tabela (Oracle)

Arquivos: `pipeline/conditions.go`, `pipeline/conditions_test.go`

---

### Fase 7 — Integracao do pipeline em `main()`

**Entregavel:** pipeline completo, equivalente ao Python.

```
strip_trailing_semicolons
→ is_merge ?
  sim: apply_merge_layout → apply_from_join_layout
  nao: preserve_block_comments → formatSQL → strip_trailing_semicolons
       → remove_table_alias_as → merge_filter_clauses
       → expand_block_comments → restore_block_comments
       → apply_and_or_layout → apply_from_join_layout → apply_limit_layout
→ finalize
```

---

### Fase 8 — Suite de testes de integracao

**Entregavel:** suite Go equivalente ao `test_format_sql.py` (50 casos).

- Testar funcoes diretamente via `testing` package
- Prioridade: MERGE, comentarios, FROM/JOIN com ON multi-linha, FILTER(WHERE...)
- Opcional: `exec.Command` no binario para testes end-to-end

Arquivo: `pipeline/integration_test.go`

---

### Fase 9 — Build, integracao com DBeaver e documentacao

**Entregavel:** substituicao do `.bat` e README atualizado.

```bat
go build -o dist\format_sql_ansi\format_sql_ansi.exe .
```

- Criar `format_sql_ansi_go.bat` apontando ao novo exe (o `.bat` Python nao e alterado)
- Atualizar `README.md` com instrucoes de build Go
- Atualizar `.gitignore` (adicionar `*.exe`)
- Atualizar `CLAUDE.md` com nova arquitetura

---

## Estrutura final de arquivos

```
sql-formatter/
  main.go
  go.mod
  go.sum
  pipeline/
    utils.go          # strip_trailing_semicolons, finalize, is_merge
    utils_test.go
    layout.go         # apply_from_join_layout, apply_limit_layout
    layout_test.go
    merge.go          # apply_merge_layout
    merge_test.go
    sqlformat.go      # substituicao do sqlglot
    sqlformat_test.go
    comments.go       # preserve/expand/restore comentarios
    comments_test.go
    conditions.go     # apply_and_or_layout, merge_filter_clauses, remove_table_alias_as
    conditions_test.go
  format_sql_ansi.bat # atualizado
  CLAUDE.md
  README.md
  .gitignore
```

Os arquivos Python (`format_sql_ansi.py`, `test_format_sql.py`) **sao mantidos permanentemente**.
O objetivo e ter duas implementacoes independentes e funcionais lado a lado:

| Implementacao | Entry point | Executavel |
|---|---|---|
| Python (atual) | `format_sql_ansi.bat` | `dist\format_sql_ansi\format_sql_ansi.exe` (PyInstaller) |
| Go (nova) | `format_sql_ansi_go.bat` | `dist\format_sql_ansi_go\format_sql_ansi_go.exe` |

Cada uma tem seu proprio `.bat`, seus proprios testes e seu proprio processo de build.
O usuario escolhe qual configurar no DBeaver.

---

## Verificacao por fase

| Fase | Como verificar |
|---|---|
| 1 | `echo SELECT 1 \| format_sql_ansi.exe` produz saida |
| 2-6 | `go test ./pipeline/...` |
| 7 | SQLs manuais: MERGE, subqueries, comentarios, FILTER |
| 8 | `go test ./...` — todos os casos passando |
| 9 | DBeaver formatando SQL real sem erros |
