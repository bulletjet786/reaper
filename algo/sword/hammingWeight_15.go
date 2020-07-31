package sword

// 请实现一个函数，输入一个整数，输出该数二进制表示中 1 的个数。
// 例如，把 9 表示成二进制是 1001，有 2 位是 1。因此，如果输入 9，则该函数输出 2。

// 时间复杂度 O(logN)
// 空间复杂度 O(1)
func hammingWeight(num uint32) int {
	count := 0
	for num != 0 {
		if num&0x1 > 0 {
			count++
		}
		num = num >> 1
	}
	return count
}

// 每次消去最右边的1
// 时间复杂度 O(M)
// 空间复杂度 O(1)
func hammingWeight2(num uint32) int {
	count := 0
	for num != 0 {
		count++
		num &= num - 1
	}
	return count
}
