package sword

// 数组中有一个数字出现的次数超过数组长度的一半，请找出这个数字。
// 你可以假设数组是非空的，并且给定的数组总是存在多数元素。

func majorityElement(nums []int) int {
	if len(nums) == 0 {
		return 0
	}

	var candidate *int
	var count int64
	for i, it := range nums {
		if candidate == nil { // 如果当前不存在候选者
			candidate = &nums[i]
			count = 1
		} else { // 如果当前存在候选者
			if it == *candidate {
				count++
			} else {
				count--
				if count == 0 {
					candidate = nil
				}
			}
		}
	}
	return *candidate
}
