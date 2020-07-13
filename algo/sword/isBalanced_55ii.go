package sword

// 深度优先遍历

// 时间复杂度：O(n)
// 空间复杂度：O(n)
func isBalanced(root *TreeNode) bool {
	balanced, _ := checkBalanceAndDepth(root)
	return balanced
}

func checkBalanceAndDepth(root *TreeNode) (balanced bool, depth int) {
	if root == nil {
		return true, 0
	}
	leftBalanced, leftDepth := checkBalanceAndDepth(root.Left)
	rightBalanced, rightDepth := checkBalanceAndDepth(root.Right)
	if leftDepth > rightDepth {
		depth = leftDepth + 1
		balanced = leftBalanced && rightBalanced && leftDepth-rightDepth <= 1
	} else {
		depth = rightDepth + 1
		balanced = leftBalanced && rightBalanced && rightDepth-leftDepth <= 1
	}
	return balanced, depth
}
