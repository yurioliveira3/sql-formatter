package pipeline

// Format executa o pipeline completo de formatação SQL.
// É o equivalente da função main() do Python, mas retorna string em vez de escrever no stdout.
//
// Pipeline para MERGE:
//   StripTrailingSemicolons → ApplyMergeLayout → ApplyFromJoinLayout → Finalize
//
// Pipeline normal:
//   StripTrailingSemicolons
//   → PreserveBlockComments → FormatSQL → StripTrailingSemicolons
//   → FixIsNotNull → RemoveTableAliasAs → MergeFilterClauses
//   → ExpandBlockComments → ConvertBlockToLineComments → RestoreBlockComments
//   → ApplyAndOrLayout → ApplyFromJoinLayout → ApplyLimitLayout
//   → Finalize
func Format(sql string) string {
	base := StripTrailingSemicolons(sql)

	if IsMerge(base) {
		base = ApplyMergeLayout(base)
		base = ApplyFromJoinLayout(base)
	} else {
		// 1. Preserva /* */ originais do usuário com placeholders
		var originalBlocks map[string]string
		base, originalBlocks = PreserveBlockComments(base)

		// 2. Formata com FormatSQL (uppercase + estrutura por cláusula)
		base = FormatSQL(base)

		// 3. Remove ; reinseridos e corrige NOT <x> IS NULL
		base = StripTrailingSemicolons(base)
		base = FixIsNotNull(base)

		// 4. Remove AS de aliases de tabela (Oracle não aceita)
		base = RemoveTableAliasAs(base)

		// 5. Reúne FILTER(WHERE ...) quebrados
		base = MergeFilterClauses(base)

		// 6. Expande /* */ em linhas de -- e converte restantes
		base = ExpandBlockComments(base)
		base = ConvertBlockToLineComments(base)

		// 7. Restaura /* */ originais do usuário
		base = RestoreBlockComments(base, originalBlocks)

		// 8. Coloca cada coluna do SELECT em sua própria linha
		base = ApplySelectLayout(base)

		// 9. Quebra AND/OR de nível superior em linhas próprias
		base = ApplyAndOrLayout(base)

		// 10. Indenta condições WHERE/HAVING
		base = ApplyWhereLayout(base)

		// 11. Aplica estilo FROM/JOIN e LIMIT
		base = ApplyFromJoinLayout(base)
		base = ApplyOrderByLayout(base)
	}

	return Finalize(base)
}
