package sword

// 从上到下打印出二叉树的每个节点，同一层的节点按照从左到右的顺序打印。

// 时间：O(N) 空间：O(N)
// BFS 通常借助 队列 的先入先出特性来实现。

func levelOrder(root *TreeNode) []int {
	if root == nil {
		return nil
	}

	res := make([]int, 0)
	level := 0
	levelNodes := make([]*TreeNode, 0)
	levelNodes = append(levelNodes, root)
	for len(levelNodes) != 0 {
		length := len(levelNodes)
		for _, it := range levelNodes {
			res = append(res, it.Val)
			// 插入新的节点
			if it.Left != nil {
				levelNodes = append(levelNodes, it.Left)
			}
			if it.Right != nil {
				levelNodes = append(levelNodes, it.Right)
			}
		}
		levelNodes = levelNodes[length:]
		level++
	}

	return res
}
