package sword

// 给定单向链表的头指针和一个要删除的节点的值，定义一个函数删除该节点。
// 返回删除后的链表的头节点。

func deleteNode(head *ListNode, val int) *ListNode {
	// 该节点是第一个节点
	if head.Val == val {
		next := head.Next
		head.Next = nil
		return next
	}

	prev := head
	cur := head.Next
	for cur != nil {
		// 如果找到了
		if cur.Val == val {
			prev.Next = cur.Next
			cur.Next = nil
			break
		}
		prev = prev.Next
		cur = cur.Next
	}

	return head
}
