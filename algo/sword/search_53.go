package sword

// 统计一个数字在排序数组中出现的次数。

// @最优解
// 时间复杂度：O(log n)
// 空间复杂度：O(1)
// 原理：二分查找找到该值的位置，然后开始迭代求出现次数
func search(nums []int, target int) int {
	// nil处理
	if nums == nil || len(nums) == 0 {
		return -1
	}

	// 未找到该元素
	insertPosition := lowerBound(nums, target)
	if insertPosition == len(nums) {
		return 0
	}

	// 找到该元素
	count := 0
	for insertPosition < len(nums) && nums[insertPosition] == target {
		insertPosition++
		count++
	}

	return count
}

// 查找在有序列表nums[first,stop)中第一个可插入位置
func lowerBound(nums []int, value int) int {
	left := 0
	right := len(nums)
	for left < right {
		mid := left + (right-left)/2
		if nums[mid] < value {
			left = mid + 1
		} else {
			right = mid
		}
	}
	return left
}
