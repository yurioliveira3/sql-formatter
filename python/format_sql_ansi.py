import re
import sys
import sqlglot

# ---------------------------------------------------------------------------
# Regex pre-compiladas no nivel de modulo
# ---------------------------------------------------------------------------

JOIN_RE = re.compile(
    r"""^(\s*)                                   # indent
        (
          (?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN
          |JOIN
        )
        \s+(.+?)\s*$                              # resto da linha
    """,
    re.IGNORECASE | re.VERBOSE,
)

FROM_RE = re.compile(r"^(\s*)FROM\s+(.+?)\s*$", re.IGNORECASE)

_TRAILING_SEMICOLONS_RE = re.compile(r"[ \t\r\n]*;+\s*\Z")
_MERGE_RE = re.compile(r"\s*MERGE\b", re.IGNORECASE)
_LONE_FROM_RE = re.compile(r"^\s*FROM\s*$", re.IGNORECASE)
_LOOSE_ON_RE = re.compile(r"^\s+ON\b", re.IGNORECASE)
_ON_WORD_RE = re.compile(r"\bON\b", re.IGNORECASE)
_MERGE_INTO_RE = re.compile(r"^MERGE\s+INTO\s+(.+)$", re.IGNORECASE)
_WHEN_MATCHED_RE = re.compile(r"^WHEN\s+(NOT\s+)?MATCHED\s+THEN", re.IGNORECASE)
_UPDATE_SET_RE = re.compile(r"^UPDATE\s+SET\b", re.IGNORECASE)
_INSERT_DELETE_RE = re.compile(r"^(INSERT|DELETE)\b", re.IGNORECASE)
_WHERE_RE = re.compile(r"^WHERE\b", re.IGNORECASE)
_FILTER_RE = re.compile(r"\bFILTER\s*\(", re.IGNORECASE)
_LIMIT_RE = re.compile(r"^\s*LIMIT\s+([^\s;]+)\s*$", re.IGNORECASE | re.MULTILINE)
_AND_OR_WORD_RE = re.compile(r"\b(AND|OR)\s", re.IGNORECASE)
_AND_OR_QUICK_RE = re.compile(r"\b(?:AND|OR)\s", re.IGNORECASE)
_INDENT_RE = re.compile(r"^(\s*)")
_BLOCK_COMMENTS_RE = re.compile(r"/\*.*?\*/", re.DOTALL)
_BLOCK_RE = re.compile(r"/\*([^\n]*?)\*/")
_KEEP_RE = re.compile(r"__KEEP_\d+__")
_STANDALONE_COMMENT_RE = re.compile(r"^[ \t]*--[^\n]*$", re.MULTILINE)
_LONE_JOIN_RE = re.compile(
    r"^\s*(?:(?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN|JOIN)\s*$",
    re.IGNORECASE,
)
_PURE_BLOCKS_RE = re.compile(r"^\s*(?:/\*[^\n]*?\*/\s*)+$")
_AND_OR_BLOCKS_RE = re.compile(
    r"^(\s*)(AND|OR)\s+(/\*[^\n]*?\*/(?:\s*/\*[^\n]*?\*/)*)\s+(\S.*)$",
    re.IGNORECASE,
)
_FROM_JOIN_TABLE_AS_RE = re.compile(
    r"^(\s*(?:(?:LEFT|RIGHT|FULL|INNER|CROSS|NATURAL)\s+(?:OUTER\s+)?JOIN|JOIN|FROM)\s+\S+)\s+AS\s+(\w+)",
    re.IGNORECASE,
)
_CLOSE_PAREN_AS_RE = re.compile(r"^(\s*\))\s+AS\s+(\w+)", re.IGNORECASE)
_NOT_IS_NULL_RE = re.compile(r"\bNOT\s+(.+?)\s+IS\s+NULL\b", re.IGNORECASE)


def strip_trailing_semicolons(sql: str) -> str:
    # Remove um ou mais ';' no fim, com espaços/linhas após
    return _TRAILING_SEMICOLONS_RE.sub("", sql)


def apply_from_join_layout(sql: str) -> str:
    """
    Reescreve linhas:
      FROM <x>   -> FROM\n\t<x>
      JOIN <x>   -> JOIN\n\t<x>
      LEFT JOIN <x> -> LEFT JOIN\n\t<x>
    Preserva indentação (útil em subqueries).
    Linhas de ON soltas (geradas pelo sqlglot) são acopladas na linha da tabela.
    """
    sql = sql.replace("\r\n", "\n").replace("\r", "\n")
    lines = sql.split("\n")
    out_lines = []
    i = 0

    while i < len(lines):
        line = lines[i]

        # Se já é só "FROM" sozinho, não mexe
        if _LONE_FROM_RE.match(line):
            out_lines.append(line.strip() if line.strip().upper() == "FROM" else line)
            i += 1
            continue

        m_from = FROM_RE.match(line)
        if m_from:
            indent, rest = m_from.group(1), m_from.group(2)
            out_lines.append(f"{indent}FROM")
            out_lines.append(f"{indent}\t{rest}")
            i += 1
            continue

        m_join = JOIN_RE.match(line)
        if m_join:
            indent, join_kw, rest = m_join.group(1), m_join.group(2), m_join.group(3)
            # Acopla linha de ON solta que o sqlglot coloca na linha seguinte
            while i + 1 < len(lines) and _LOOSE_ON_RE.match(lines[i + 1]):
                i += 1
                rest = rest + " " + lines[i].strip()
            # Se o ON tem parênteses abertos, continua acoplando até fechar
            on_pos = _ON_WORD_RE.search(rest)
            if on_pos:
                s = rest[on_pos.start():]
                depth = s.count("(") - s.count(")")
                while depth > 0 and i + 1 < len(lines):
                    i += 1
                    cont = lines[i].strip()
                    rest = rest + " " + cont
                    depth += cont.count("(") - cont.count(")")
            out_lines.append(f"{indent}{join_kw.strip()}")
            out_lines.append(f"{indent}\t{rest}")
            i += 1
            continue

        out_lines.append(line)
        i += 1

    return "\n".join(out_lines)


def apply_limit_layout(sql: str) -> str:
    # LIMIT 5 -> LIMIT\n\t5
    return _LIMIT_RE.sub(r"LIMIT\n\t\1", sql)


def finalize(sql: str) -> str:
    sql = sql.replace("\r\n", "\n").replace("\r", "\n")
    sql = strip_trailing_semicolons(sql).rstrip()
    return sql + "\n;\n"


def is_merge(sql: str) -> bool:
    return bool(_MERGE_RE.match(sql))


def apply_merge_layout(sql: str) -> str:
    """
    Aplica o layout padrão para MERGE:
      MERGE INTO <tabela> <alias>
      USING (
          SELECT ...
          FROM
              <tabela> <alias>
      ) <alias> ON (<join>)
      WHEN [NOT] MATCHED THEN
          UPDATE SET / INSERT / DELETE
    """
    sql = sql.replace("\r\n", "\n").replace("\r", "\n")
    out = []

    for line in sql.split("\n"):
        stripped = line.strip()

        if not stripped:
            out.append("")
            continue

        # MERGE INTO <table> [alias]  →  MERGE INTO\n\t<table> [alias]
        m = _MERGE_INTO_RE.match(stripped)
        if m:
            out.append("MERGE INTO")
            out.append(f"\t{m.group(1).strip()}")
            continue

        # WHEN [NOT] MATCHED THEN  →  sem indentação
        if _WHEN_MATCHED_RE.match(stripped):
            out.append(stripped)
            continue

        # UPDATE SET  →  \tUPDATE SET
        if _UPDATE_SET_RE.match(stripped):
            out.append(f"\tUPDATE SET")
            continue

        # INSERT / DELETE dentro do WHEN  →  \t<keyword> ...
        if _INSERT_DELETE_RE.match(stripped):
            out.append(f"\t{stripped}")
            continue

        # Assignments e WHERE dentro do bloco WHEN  →  \t<linha>
        if _WHERE_RE.match(stripped):
            out.append(f"\tWHERE")
            continue

        out.append(line)

    return "\n".join(out)


def split_top_level_and_or(line: str) -> list[str]:
    """
    Divide uma linha nos AND/OR de nível superior (fora de parênteses e strings).
    Retorna lista com as partes; se não houver split, retorna lista com a linha original.

    Usa finditer + array de profundidade para evitar O(n * custo_regex) do re.match
    por posição de caractere.
    """
    n = len(line)
    depth_at = [0] * n
    # skip_at: True para posições dentro de strings ou na aspa de abertura
    skip_at = [False] * n
    depth = 0
    in_string = False
    string_char = ""

    for i, ch in enumerate(line):
        if in_string:
            skip_at[i] = True
            if ch == string_char:
                in_string = False
        elif ch in ("'", '"'):
            skip_at[i] = True
            in_string = True
            string_char = ch
        else:
            if ch == "(":
                depth += 1
            elif ch == ")":
                depth -= 1
        depth_at[i] = depth

    split_points = [
        m.start()
        for m in _AND_OR_WORD_RE.finditer(line)
        if depth_at[m.start()] == 0 and not skip_at[m.start()]
    ]

    if not split_points:
        return [line]

    parts = []
    prev = 0
    for pos in split_points:
        parts.append(line[prev:pos].rstrip())
        prev = pos
    parts.append(line[prev:])

    return parts


def apply_and_or_layout(sql: str) -> str:
    """
    Quebra condições AND/OR de nível superior em linhas próprias.
    Ex:  WHERE a = 1 AND b = 2  →  WHERE / <indent>a = 1 / <indent>AND b = 2
    Respeita parênteses e strings — não quebra dentro de ON (...) ou FILTER(...).
    Linhas de comentário (--) nunca são quebradas.
    """
    lines = sql.split("\n")
    out = []
    for line in lines:
        # Short-circuit: ignora linhas sem AND/OR (a maioria das linhas)
        if not _AND_OR_QUICK_RE.search(line):
            out.append(line)
            continue
        # Linhas de comentário -- nunca devem ser quebradas
        if line.lstrip().startswith("--"):
            out.append(line)
            continue
        indent = _INDENT_RE.match(line).group(1)
        parts = split_top_level_and_or(line)
        if len(parts) > 1:
            for part in parts:
                stripped = part.strip()
                if stripped:
                    out.append(f"{indent}{stripped}")
        else:
            out.append(line)
    return "\n".join(out)


def merge_filter_clauses(sql: str) -> str:
    """
    Reúne linhas de FILTER(WHERE ...) que o sqlglot quebra em múltiplas linhas.
    Conta parênteses para saber quando o FILTER( foi fechado.
    """
    lines = sql.split("\n")
    out = []
    i = 0
    while i < len(lines):
        line = lines[i]
        m = _FILTER_RE.search(line)
        if m:
            s = line[m.start():]
            depth = s.count("(") - s.count(")")
            while depth > 0 and i + 1 < len(lines):
                i += 1
                continuation = lines[i].strip()
                line = line + " " + continuation
                depth += continuation.count("(") - continuation.count(")")
        out.append(line)
        i += 1
    return "\n".join(out)


def preserve_block_comments(sql: str) -> tuple[str, dict]:
    """
    Substitui /* */ originais do usuário por placeholders antes do sqlglot,
    para que não sejam convertidos para -- depois.
    Retorna (sql_modificado, {placeholder: comentário_original}).
    """
    placeholders = {}
    counter = [0]

    def replace(m):
        key = f"/*__KEEP_{counter[0]}__*/"
        placeholders[key] = m.group(0)
        counter[0] += 1
        return key

    modified = _BLOCK_COMMENTS_RE.sub(replace, sql)
    return modified, placeholders


def strip_join_comments(sql: str) -> tuple[str, list]:
    """
    Remove APENAS linhas -- que aparecem imediatamente após uma keyword JOIN
    sozinha numa linha (ex: JOIN\\n-- RP ORIGEM\\ntabela ON ...).
    sqlglot embute esses comentários no meio do identificador schema.tabela,
    corrompendo o resultado.

    Retorna (sql_modificado, [(comment_text, next_fragment)])
    onde next_fragment é o início normalizado da próxima linha concreta.
    """
    lines = sql.split("\n")
    stored = []
    clean = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if (
            line.strip().startswith("--")
            and clean
            and _LONE_JOIN_RE.match(clean[-1])
        ):
            # Find the next non-empty, non-comment line (the table name)
            j = i + 1
            while j < len(lines) and (
                not lines[j].strip() or lines[j].strip().startswith("--")
            ):
                j += 1
            next_fragment = " ".join(lines[j].split())[:30] if j < len(lines) else ""
            stored.append((line.strip(), next_fragment))
        else:
            clean.append(line)
        i += 1
    return "\n".join(clean), stored


def restore_standalone_comments(sql: str, stored: list) -> str:
    """
    Reinsere linhas -- de JOIN (removidas por strip_join_comments) antes da
    linha da tabela correspondente. Usa o fragmento inicial da tabela como chave.
    """
    for comment, fragment in stored:
        if not fragment:
            continue
        lines = sql.split("\n")
        norm_frag = fragment[:20]
        for i, line in enumerate(lines):
            norm_line = " ".join(line.strip().split())
            if norm_line.startswith(norm_frag):
                # Copia a indentação da linha de referência
                indent = ""
                for ch in line:
                    if ch in (" ", "\t"):
                        indent += ch
                    else:
                        break
                lines.insert(i, indent + comment)
                sql = "\n".join(lines)
                break
    return sql


def restore_block_comments(sql: str, placeholders: dict) -> str:
    """
    Restaura os /* */ originais a partir dos placeholders.
    Usa regex para tolerar espaços que o sqlglot adiciona ao redor do conteúdo.
    """
    for key, original in placeholders.items():
        content = re.escape(key[2:-2])  # conteúdo entre /* e */
        sql = re.sub(r"/\*\s*" + content + r"\s*\*/", original, sql)
    return sql


def expand_block_comments(sql: str) -> str:
    """
    Separa /* comentários */ inline em linhas próprias e converte para --.
    (Incorpora a lógica de convert_block_to_line_comments em um único passo.)

    Padrão A — linha inteira são blocos /* */:
      /* col1 */ /* col2 */  →  -- col1 / -- col2  (uma por linha)

    Padrão B — AND/OR + blocos + SQL real:
      AND /* cond1 */ /* cond2 */ expr  →  -- cond1 / -- cond2 / AND expr

    Padrão C — código seguido de 2+ blocos no final da linha:
      lin.* /* col1 */ /* col2 */  →  lin.* / -- col1 / -- col2

    Fallback — blocos restantes: /* comentário */ → -- comentário
    (placeholders __KEEP__ são preservados intactos em todos os casos)
    """
    lines = sql.split("\n")
    out = []

    for line in lines:
        indent = _INDENT_RE.match(line).group(1)

        # Padrões A/B/C não se aplicam a linhas com placeholders
        if not _KEEP_RE.search(line):
            # Padrão B: AND/OR seguido de bloco(s) de comentário e SQL real
            m = _AND_OR_BLOCKS_RE.match(line)
            if m:
                pre_indent, keyword, comments_str, actual = (
                    m.group(1), m.group(2), m.group(3), m.group(4)
                )
                for cm in _BLOCK_RE.finditer(comments_str):
                    out.append(f"{pre_indent}-- {cm.group(1).strip()}")
                out.append(f"{pre_indent}{keyword} {actual}")
                continue

            # Padrão A: linha inteira composta por blocos de comentário
            if _PURE_BLOCKS_RE.match(line):
                for cm in _BLOCK_RE.finditer(line):
                    out.append(f"{indent}-- {cm.group(1).strip()}")
                continue

            # Padrão C: código seguido de 2+ blocos de comentário no final da linha
            trailing = list(_BLOCK_RE.finditer(line))
            if len(trailing) >= 2:
                last_end = trailing[-1].end()
                if last_end >= len(line.rstrip()):
                    code_part = line[:trailing[0].start()].rstrip()
                    if code_part.strip():
                        out.append(code_part)
                        for cm in trailing:
                            out.append(f"{indent}-- {cm.group(1).strip()}")
                        continue

        # Fallback: converte blocos /* */ restantes para -- (preserva __KEEP__)
        out.append(_BLOCK_RE.sub(
            lambda m: m.group(0) if _KEEP_RE.search(m.group(1)) else "-- " + m.group(1).strip(),
            line,
        ))

    return "\n".join(out)


def remove_table_alias_as(sql: str) -> str:
    """
    Remove AS de aliases de tabela gerados pelo sqlglot, que o Oracle não aceita.
    Cobre FROM/JOIN e aliases de subquery (') AS alias').
    Preserva AS em aliases de coluna (SELECT col AS alias).
    """
    lines = sql.split("\n")
    out = []
    for line in lines:
        m = _FROM_JOIN_TABLE_AS_RE.match(line)
        if m:
            line = m.group(1) + " " + m.group(2) + line[m.end():]
        else:
            m2 = _CLOSE_PAREN_AS_RE.match(line)
            if m2:
                line = m2.group(1) + " " + m2.group(2) + line[m2.end():]
        out.append(line)
    return "\n".join(out)


def fix_is_not_null(sql: str) -> str:
    """
    Reverte a transformação do sqlglot que converte IS NOT NULL para NOT <expr> IS NULL.
    """
    return _NOT_IS_NULL_RE.sub(lambda m: f"{m.group(1)} IS NOT NULL", sql)


def try_sqlglot(sql: str) -> str | None:
    try:
        statements = sqlglot.transpile(sql, pretty=True)
        if not statements:
            return None
        return "\n;\n".join(statements)
    except sqlglot.errors.SqlglotError:
        return None


def main():
    raw = sys.stdin.read()

    base = strip_trailing_semicolons(raw)

    if is_merge(base):
        # MERGE: não passa pelo sqlglot (sem suporte genérico)
        # aplica layout customizado + FROM/JOIN no interior
        base = apply_merge_layout(base)
        base = apply_from_join_layout(base)
    else:
        # 1a) remove linhas -- entre keyword JOIN e tabela (sqlglot as mangle)
        base, standalone_comments = strip_join_comments(base)

        # 1b) preserva /* */ originais do usuário com placeholders
        base, original_blocks = preserve_block_comments(base)

        # 2) formata com sqlglot
        formatted = try_sqlglot(base)
        base = formatted if formatted is not None else base

        # 3) remove ; reinseridos pelo sqlglot antes de mexer em linhas
        base = strip_trailing_semicolons(base)

        # 3b) reverte NOT <expr> IS NULL → <expr> IS NOT NULL
        base = fix_is_not_null(base)

        # 3d) remove AS de aliases de tabela (Oracle não aceita)
        base = remove_table_alias_as(base)

        # 4) reúne FILTER(WHERE ...) quebrados pelo sqlglot
        base = merge_filter_clauses(base)

        # 5) expande /* */ (de --) em linhas próprias e converte para --
        #    (único passo; incorpora a conversão bloco→linha)
        base = expand_block_comments(base)

        # 6) restaura /* */ originais do usuário
        base = restore_block_comments(base, original_blocks)

        # 7) quebra AND/OR de nível superior em linhas próprias
        base = apply_and_or_layout(base)

        # 8) aplica estilo FROM/JOIN/LIMIT
        base = apply_from_join_layout(base)
        base = apply_limit_layout(base)

        # 9) reinsere linhas -- de JOIN removidas antes do sqlglot
        base = restore_standalone_comments(base, standalone_comments)

    # garante 1 ';' no final
    sys.stdout.write(finalize(base))


if __name__ == "__main__":
    main()
