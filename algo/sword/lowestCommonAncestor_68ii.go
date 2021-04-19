package sword

type Result struct {
	Find bool
	Node *TreeNode
}

func lowestCommonAncestor(root *TreeNode, p, q int) *TreeNode {
	var result = &Result{
		Find: false,
		Node: nil,
	}
	lowestCommonAncestorInner(root, p, q, result)
	if result.Find {
		return result.Node
	} else {
		return nil
	}
}

func lowestCommonAncestorInner(root *TreeNode, p, q int, result *Result) (bool, bool) {
	if result.Find {
		return true, true
	}
	if root == nil {
		return false, false
	}
	leftPResult, leftQResult := false, false
	rightPResult, rightQResult := false, false
	if root.Left != nil {
		leftPResult, leftQResult = lowestCommonAncestorInner(root.Left, p, q, result)
	}
	if root.Right != nil {
		rightPResult, rightQResult = lowestCommonAncestorInner(root.Right, p, q, result)
	}
	pResult := leftPResult || rightPResult || (root.Val == p)
	qResult := leftQResult || rightQResult || (root.Val == q)
	if pResult && qResult && !result.Find {
		result.Node = root
		result.Find = true
		return true, true
	}
	return pResult, qResult
}
