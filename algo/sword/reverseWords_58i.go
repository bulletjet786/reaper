package sword

import "strings"

// 输入一个英文句子，翻转句子中单词的顺序，但单词内字符的顺序不变。为简单起见，标点符号和普通字母一样处理。
// 例如输入字符串"I am a student. "，则输出"student. a am I"。

// 算法解析：
// 倒序遍历字符串 s ，记录单词左右索引边界 i , j ；
// 每确定一个单词的边界，则将其添加至单词列表 res ；
// 最终，将单词列表拼接为字符串，并返回即可。

// 算法复杂度
// 时间复杂度 O(N) ： 其中 N 为字符串 s 的长度，线性遍历字符串。
// 空间复杂度 O(N) ： 新建的 list(Python) 或 StringBuilder(Java) 中的字符串总长度 N ，占用 O(N)大小的额外空间。
func reverseWords(s string) string {
	trimed := strings.Trim(s, " ")
	var res []string
	var i = len(trimed) - 1
	var stop = len(trimed) - 1
	for i >= 0 {
		for i >= 0 && trimed[i] != ' ' {
			i--
		}
		res = append(res, trimed[i+1:stop+1])
		for i >= 0 && trimed[i] == ' ' {
			i--
		}
		stop = i
	}
	return strings.Join(res, " ")
}
