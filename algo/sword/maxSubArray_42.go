package sword

// 输入一个整型数组，数组中的一个或连续多个整数组成一个子数组。求所有子数组的和的最大值。
// 要求时间复杂度为O(n)。

func maxSubArray(nums []int) int {
	const Int32Max = int(^uint(0) >> 1)
	const Int32Min = ^Int32Max

	if len(nums) == 0 {
		return 0
	}
	maxSubSum := Int32Min
	subSum := 0
	for _, it := range nums {
		subSum += it
		if subSum > maxSubSum {
			maxSubSum = subSum
		}
		if subSum < 0 {
			subSum = 0
		}
	}
	return maxSubSum
}
