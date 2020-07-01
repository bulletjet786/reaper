package sword

// 找出数组中重复的数字。

/* 在一个长度为 n 的数组 nums 里的所有数字都在 0～n-1 的范围内。
 * 数组中某些数字是重复的，但不知道有几个数字重复了，也不知道每个数字重复了几次。请找出数组中任意一个重复的数字。
 */

// @最优解
// 时间复杂度：O(n)
// 空间复杂度：O(n) - @拷贝
// 原理：原地置换
func findRepeatNumber(nums []int) int {
	// nil处理
	if nums == nil || len(nums) == 0 {
		return -1
	}

	arr := make([]int, len(nums))
	// 初始化数据: 此时arr[0] == 0，原因是使用了数据的默认值，理论上可全部初始化为-1
	arr[0] = -1
	for _, n := range nums {
		// 如果当前位置同位数据，则该数据为重复数据
		if arr[n] == n {
			return n
		}
		arr[n] = n
	}
	return -1
}

// 时间复杂度：O(n)
// 空间复杂度：O(n) - @辅助
// 原理：map计数
func findRepeatNumber2(nums []int) int {
	counter := make(map[int]int, len(nums))
	for _, n := range nums {
		if counter[n] > 0 {
			return n
		}
		counter[n]++
	}
	return -1
}
