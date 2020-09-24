package sword

func firstUniqChar(s string) byte {
	show := make([]int, 26)
	for _, it := range s {
		show[it-'a']++
	}
	for _, it := range s {
		if show[it-'a'] == 1 {
			return byte(it)
		}
	}
	return ' '
}
