package sword

// 深度优先遍历

// 时间复杂度 O(N) ： N 为树的节点数量，计算树的深度需要遍历所有节点。
// 空间复杂度 O(N) ： 最差情况下（当树退化为链表时），递归深度可达到 N 。
func maxDepth(root *TreeNode) int {
	if root == nil {
		return 0
	}
	left := maxDepth(root.Left)
	right := maxDepth(root.Right)
	if left > right {
		return left + 1
	} else {
		return right + 1
	}
}
