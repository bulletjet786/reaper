package sword

// 给定一棵二叉搜索树，请找出其中第k大的节点。

// 根据以上性质，易得二叉搜索树的 中序遍历倒序 为 递减序列 。
// 因此，求 “二叉搜索树第 k 大的节点” 可转化为求 “此树的中序遍历倒序的第 k 个节点”。

func kthLargest(root *TreeNode, k int) int {
	if root == nil {
		return 0
	}

	now := 0
	res := 0
	found := false
	foundP := &found
	var resPointer = &res
	walk(root, k, &now, resPointer, foundP)
	return *resPointer
}

func walk(root *TreeNode, count int, now *int, res *int, found *bool) {
	if root == nil {
		return
	}
	if *found {
		return
	}
	walk(root.Right, count, now, res, found)
	*now++
	if *now == count {
		*found = true
		*res = root.Val
		return
	}
	walk(root.Left, count, now, res, found)
}
