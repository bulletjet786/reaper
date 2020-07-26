package sword

// 请实现一个函数，把字符串 s 中的每个空格替换成"%20"。

func replaceSpace(s string) string {
	var res string
	for _, c := range s {
		if c == ' ' {
			res = res + "%20"
		} else {
			res = res + string(c)
		}
	}
	return res
}
