package sword

// 先对所有数字进行一次异或，得到两个出现一次的数字的异或值。
// 在异或结果中找到任意为 1 的位。
// 根据这一位对所有的数字进行分组。
// 在每个组内进行异或操作，得到两个数字。
func singleNumbers(nums []int) []int {
	twoXor := 0
	for _, it := range nums {
		twoXor ^= it
	}
	var mask = 1
	for twoXor&mask == 0 {
		mask <<= 1
	}
	var maskNum = 0
	var unmaskNum = 0
	for _, it := range nums {
		if it&mask == 0 {
			maskNum ^= it
		} else {
			unmaskNum ^= it
		}
	}
	return []int{maskNum, unmaskNum}
}
