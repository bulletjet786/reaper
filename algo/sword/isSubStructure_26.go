package sword

// 输入两棵二叉树A和B，判断B是不是A的子结构。(约定空树不是任意一个树的子结构)
// B是A的子结构， 即 A中有出现和B相同的结构和节点值。

// 1. 先序遍历树A中的每个节点t
// 2. 判断树A中以t为根节点的子树是否包含树B

// M:A,N:B -> 时间: O(MN) 空间: O(M)
func isSubStructure(a *TreeNode, b *TreeNode) bool {
	if a == nil || b == nil {
		return false
	}
	return isSameTop(a, b) || isSubStructure(a.Left, b) || isSubStructure(a.Right, b)
}

func isSameTop(a *TreeNode, b *TreeNode) bool {
	if b == nil {
		return true
	}
	// 顶部是否一样
	if a == nil || a.Val != b.Val {
		return false
	}
	return isSameTop(a.Left, b.Left) && isSameTop(a.Right, b.Right)
}
