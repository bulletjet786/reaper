package sword

type ListNode struct {
	Val  int
	Next *ListNode
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
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
