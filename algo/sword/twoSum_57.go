package sword

// 输入一个递增排序的数组和一个数字s，在数组中查找两个数，使得它们的和正好是s。
// 如果有多对数字的和等于s，则输出任意一对即可。

// 实例：nums = [10,27,30,31,47,60], target = 58

// 双指针 i , j 分别指向数组 nums 的左右两端，向中间靠拢
// 正确性证明：
// 状态 S(i, j)S(i,j) 切换至 S(i + 1, j)S(i+1,j) ，则会消去一行元素，相当于 消去了状态集合 {S(i, i + 1), S(i, i + 2), ..., S(i, j - 2), S(i, j - 1), S(i, j)S(i,i+1),S(i,i+2),...,S(i,j−2),S(i,j−1),S(i,j) } 。
// 由于双指针都是向中间收缩，因此这些状态之后不可能再遇到）。
// 由于 nums 是排序数组，因此这些 消去的状态 都一定满足 S(i, j) < targetS(i,j)<target ，即这些状态都 不是解 。
// 结论： 以上分析已证明 “每次指针 ii 的移动操作，都不会导致解的丢失” ，即指针 ii 的移动操作是 安全的 ；同理，对于指针 jj 可得出同样推论；因此，此双指针法是正确的。
func twoSum(nums []int, target int) []int {
	i := 0
	j := len(nums) - 1
	for i < j {
		if nums[i]+nums[j] == target {
			return []int{nums[i], nums[j]}
		} else if nums[i]+nums[j] < target {
			i++
		} else {
			j--
		}
	}
	return nil
}
