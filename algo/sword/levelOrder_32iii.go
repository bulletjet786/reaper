package sword

// 请实现一个函数按照之字形顺序打印二叉树，即第一行按照从左到右的顺序打印，
// 第二层按照从右到左的顺序打印，第三行再按照从左到右的顺序打印，其他行以此类推。

// 偶数层倒序： 若 res 的长度为 奇数 ，说明当前是偶数层，则对 tmp 执行 倒序 操作

// 空间：O(n)
// 时间：O(n)
func levelOrder3(root *TreeNode) [][]int {
	if root == nil {
		return nil
	}
	var list []*TreeNode
	list = append(list, root)
	reverse := false
	res := make([][]int, 0)
	for len(list) != 0 {
		length := len(list)
		output := make([]int, 0)
		for _, it := range list {
			output = append(output, it.Val)
			// 插入新的节点
			if it.Left != nil {
				list = append(list, it.Left)
			}
			if it.Right != nil {
				list = append(list, it.Right)
			}
		}

		if reverse {
			for i, j := 0, len(output)-1; i < j; i, j = i+1, j-1 {
				output[i], output[j] = output[j], output[i]
			}
		}

		list = list[length:]
		reverse = !reverse
		res = append(res, output)
	}
	return res
}
