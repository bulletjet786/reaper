package sword

// 请完成一个函数，输入一个二叉树，该函数输出它的镜像。

// 方法一：递归法
// O(n) O(n)
//根据二叉树镜像的定义，考虑递归遍历（dfs）二叉树，交换每个节点的左 / 右子节点，即可生成二叉树的镜像。
//递归解析：
//终止条件： 当节点 rootroot 为空时（即越过叶节点），则返回 nullnull ；
//递推工作：
//初始化节点 tmptmp ，用于暂存 rootroot 的左子节点；
//开启递归 右子节点 mirrorTree(root.right)mirrorTree(root.right) ，并将返回值作为 rootroot 的 左子节点 。
//开启递归 左子节点 mirrorTree(tmp)mirrorTree(tmp) ，并将返回值作为 rootroot 的 右子节点 。

// 方法二：辅助栈（或队列）
// O(n) O(n)
//利用栈（或队列）遍历树的所有节点 nodenode ，并交换每个 nodenode 的左 / 右子节点。
//算法流程：
//特例处理： 当 rootroot 为空时，直接返回 nullnull ；
//初始化： 栈（或队列），本文用栈，并加入根节点 rootroot 。
//循环交换： 当栈 stackstack 为空时跳出；
//出栈： 记为 nodenode ；
//添加子节点： 将 nodenode 左和右子节点入栈；
//交换： 交换 nodenode 的左 / 右子节点。
//返回值： 返回根节点 rootroot 。

func mirrorTree(root *TreeNode) *TreeNode {
	if root == nil {
		return root
	}
	root.Right, root.Left = mirrorTree(root.Left), mirrorTree(root.Right)
	return root
}

func mirrorTree2(root *TreeNode) *TreeNode {
	if root == nil {
		return root
	}
	nodes := make([]*TreeNode, 0)
	nodes = append(nodes, root)
	for len(nodes) != 0 {
		node := nodes[0]
		if node.Left != nil {
			nodes = append(nodes, node.Left)
		}
		if node.Right != nil {
			nodes = append(nodes, node.Right)
		}
		node.Left, node.Right = node.Right, node.Left
		nodes = nodes[1:]
	}
	return root
}
