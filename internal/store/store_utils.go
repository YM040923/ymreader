package store

// firstString 从可变参数中安全取第一个字符串值。
func firstString(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}
