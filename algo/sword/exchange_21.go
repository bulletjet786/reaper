package sword

// 输入一个整数数组，实现一个函数来调整该数组中数字的顺序，使得所有奇数位于数组的前半部分，所有偶数位于数组的后半部分。

// 首尾指针
func exchange(nums []int) []int {
	i := 0
	j := len(nums) - 1
	for i < j {
		// 找偶数
		if nums[i]%2 != 0 {
			i++
			continue
		}
		// 找奇数
		if nums[j]%2 != 1 {
			j--
			continue
		}
		nums[i], nums[j] = nums[j], nums[i]
	}
	return nums
}

// 快慢指针
// fast 的作用是向前搜索奇数位置，low 的作用是指向下一个奇数应当存放的位置
// fast 向前移动，当它搜索到奇数时，将它和 nums[low] 交换，此时 low 向前移动一个位置.
// [0,low)都是奇数

func exchange2(nums []int) []int {
	slow := 0
	for fast, it := range nums {
		if it%2 == 1 {
			nums[fast], nums[slow] = nums[slow], nums[fast]
			slow++
		}
	}
	return nums
}
