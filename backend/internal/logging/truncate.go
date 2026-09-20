package logging

// MaxTruncateRunes は内部計装（args/result）・外部レスポンスボディを丸める
// 既定の上限文字数（rune 単位）。HTTP ボディの上限は middleware 側の別定数。
const MaxTruncateRunes = 1000

// truncateEllipsis は丸め時に付与する省略記号。
const truncateEllipsis = "…"

// Truncate は文字列を先頭 n 文字（rune 単位）に丸め、超過時は省略記号を付す。
// n 以下ならそのまま返す。
func Truncate(s string, n int) string {
	if n < 0 {
		n = 0
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + truncateEllipsis
}
