package sword

// 输入两个递增排序的链表，合并这两个链表并使新链表中的节点仍然是递增排序的。

// 时间O(1), 空间O （M+N）
// 引入伪头节点： 由于初始状态合并链表中无节点，因此循环第一轮时无法将节点添加到合并链表中。
// 解决方案：初始化一个辅助节点 dumdum 作为合并链表的伪头节点，将各节点添加至 dumdum 之后。

func mergeTwoLists(l1 *ListNode, l2 *ListNode) *ListNode {
	headDummy := &ListNode{}
	var cur = headDummy
	for l1 != nil || l2 != nil {
		cur.Next = &ListNode{}
		if l1 == nil {
			cur.Next.Val = l2.Val
			l2 = l2.Next
		} else if l2 == nil {
			cur.Next.Val = l1.Val
			l1 = l1.Next
		} else {
			if l1.Val < l2.Val {
				cur.Next.Val = l1.Val
				l1 = l1.Next
			} else {
				cur.Next.Val = l2.Val
				l2 = l2.Next
			}
		}
		cur = cur.Next
	}
	return headDummy.Next
}
