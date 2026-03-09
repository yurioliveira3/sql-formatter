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

## Abordagem: TDD

Cada fase segue o ciclo Red → Green → Refactor:
1. Escrever os testes primeiro (falham — Red)
2. Implementar o minimo para os testes passarem (Green)
3. Refatorar se necessario, mantendo os testes verdes

O `python/test_format_sql.py` e lido como referencia de comportamento esperado,
nao como limite de cobertura. A suite Go deve:
- Cobrir todos os comportamentos ja validados pelo pytest (mesma logica, inputs equivalentes)
- Adicionar casos novos para cenarios nao cobertos, edge cases e combinacoes mais complexas

O objetivo e ter uma suite Go mais completa do que a Python, nao uma copia dela.

---

## Fases de implementacao

### Fase 1 — Skeleton do projeto Go

**Entregavel:** binario que le stdin e escreve stdout sem processar.

Nao ha logica a testar nesta fase — apenas infraestrutura:
- `go mod init github.com/yurioliveira3/sql-formatter`
- `main.go` com leitura de `os.Stdin` e escrita em `os.Stdout`
- `go build -o format_sql_ansi.exe .`
- Atualizar `.gitignore` (adicionar `*.exe`)

Arquivos criados: `go.mod`, `main.go`

---

### Fase 2 — Utilitarios e funcoes simples

**Ciclo TDD:**
1. Ler `python/test_format_sql.py` como referencia de comportamento
2. Escrever `pipeline/utils_test.go` e `pipeline/merge_test.go` cobrindo os casos existentes e adicionando novos (inputs vazios, multiplos `;`, MERGE com variantes nao testadas no Python, etc.)
3. `go test ./pipeline/...` → falha (funcoes nao existem)
4. Implementar `pipeline/utils.go` e `pipeline/merge.go`
5. `go test ./pipeline/...` → verde

Funcoes cobertas:
- `strip_trailing_semicolons`
- `finalize`
- `is_merge`
- `apply_limit_layout`
- `apply_merge_layout`

---

### Fase 3 — Layout FROM/JOIN

**Ciclo TDD:**
1. Ler os casos de `apply_from_join_layout` em `python/test_format_sql.py` como referencia
2. Escrever `pipeline/layout_test.go` cobrindo os casos existentes e adicionando novos (multiplos JOINs, FROM em subquery aninhada, ON com expressoes complexas, etc.)
3. `go test ./pipeline/...` → falha
4. Implementar `pipeline/layout.go`
5. `go test ./pipeline/...` → verde

---

### Fase 4 — Substituicao do sqlglot (ponto critico)

**Ciclo TDD:**
1. Escrever `pipeline/sqlformat_test.go` com SQLs de entrada (lowercase/misturado) e saida esperada (keywords uppercase, estrutura basica)
2. `go test ./pipeline/...` → falha
3. Implementar `pipeline/sqlformat.go`: regex de keyword uppercasing + normalizacao
4. Iterar ate os testes passarem

---

### Fase 5 — Handling de comentarios

**Ciclo TDD:**
1. Escrever `pipeline/comments_test.go` com os 3 padroes (A, B, C) + preserve/restore
2. `go test ./pipeline/...` → falha
3. Implementar `pipeline/comments.go`
4. `go test ./pipeline/...` → verde

---

### Fase 6 — AND/OR layout e FILTER

**Ciclo TDD:**
1. Escrever `pipeline/conditions_test.go`: AND/OR top-level, dentro de parenteses, FILTER(WHERE...), remove AS de tabela
2. `go test ./pipeline/...` → falha
3. Implementar `pipeline/conditions.go`
4. `go test ./pipeline/...` → verde

---

### Fase 7 — Integracao do pipeline em `main()`

**Ciclo TDD:**
1. Escrever `pipeline/integration_test.go` com os 50 casos do `test_format_sql.py` (input/output completos)
2. `go test ./...` → falha (pipeline nao montado)
3. Montar o pipeline em `main.go`:

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

4. `go test ./...` → verde

---

### Fase 8 — Build, integracao com DBeaver e documentacao

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
