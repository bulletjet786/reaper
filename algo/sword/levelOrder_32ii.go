package sword

// 从上到下按层打印二叉树，同一层的节点按从左到右的顺序打印，每一层打印到一行。

// 时间：O(N) 空间：O(N)
// I. 按层打印： 题目要求的二叉树的 从上至下 打印（即按层打印），又称为二叉树的 广度优先搜索（BFS）。BFS 通常借助 队列 的先入先出特性来实现。
// II. 每层打印到一行： 将本层全部节点打印到一行，并将下一层全部节点加入队列，以此类推，即可分为多行打印。

func levelOrder2(root *TreeNode) [][]int {
	if root == nil {
		return nil
	}

	res := make([][]int, 0)
	level := 0
	levelNodes := make([]*TreeNode, 0)
	levelNodes = append(levelNodes, root)
	for len(levelNodes) != 0 {
		length := len(levelNodes)
		res = append(res, make([]int, 0))
		for _, it := range levelNodes {
			res[level] = append(res[level], it.Val)
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
