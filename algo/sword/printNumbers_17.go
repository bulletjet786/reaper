package sword

import "math"

// 输入数字 n，按顺序打印出从 1 到最大的 n 位十进制数。比如输入 3，则打印出 1、2、3 一直到最大的 3 位数 999。

func printNumbers(n int) []int {
	if n <= 0 {
		return nil
	}

	limit := int(math.Pow(10, float64(n)) - 1)
	res := make([]int, limit)
	for i := 0; i < len(res); i++ {
		res[i] = i + 1
	}
	return res
}
