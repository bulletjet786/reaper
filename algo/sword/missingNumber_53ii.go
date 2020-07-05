package sword

// 0～n-1中缺失的数字
// @最优解
// 时间复杂度：O(log n)
// 空间复杂度：O(1)
// 原理：二分查找，每次筛选出一半错误的，注意循环不变式
func missingNumber(nums []int) int {
	left := 0
	right := len(nums)
	// 循环不变式：
	// i < left 时, nums[i]=i
	// i > right 时, nums[i]=i+1
	for left < right {
		mid := left + (right-left)/2
		if nums[mid] > mid {
			// 缺失值在左边
			right = mid
		} else {
			left = mid + 1
		}
	}
	return left
}
