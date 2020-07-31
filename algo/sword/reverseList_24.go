package sword

// 定义一个函数，输入一个链表的头节点，反转该链表并输出反转后链表的头节点。

// 双指针旋转
//定义两个指针： next 和 cur ；next 在前 cur 在后。
//每次让 next 的 Next 指向 cur ，实现一次局部反转
//局部反转完成之后， next 和 cur 同时往前移动一个位置
//循环上述过程，直至 next 到达链表尾部
func reverseList(head *ListNode) *ListNode {
	var cur *ListNode = nil
	var next = head

	for next != nil {
		t := next.Next
		next.Next = cur
		cur = next
		next = t
	}

	return cur
}

// 使用递归函数，一直递归到链表的最后一个结点，该结点就是反转后的头结点，记作 ret .
//此后，每次函数在返回的过程中，让当前结点的下一个结点的 Next 指针指向当前节点。
//同时让当前结点的 Next 指针指向 NULL ，从而实现从链表尾部开始的局部反转
//当递归函数全部出栈后，链表反转完成。
func reverseList2(head *ListNode) *ListNode {
	if head == nil || head.Next == nil {
		return head
	}

	reserved := reverseList2(head.Next)
	head.Next.Next = head
	reserved.Next = nil

	return head
}

// 头插法
//原链表的头结点就是反转之后链表的尾结点，使用 head 标记 .
//定义指针 cur，初始化为 head .
//每次都让 head 下一个结点的 Next 指向 cur ，实现一次局部反转
//局部反转完成之后，cur 和 head 的 Next 指针同时 往前移动一个位置
//循环上述过程，直至 cur 到达链表的最后一个结点 .
func reverseList3(head *ListNode) *ListNode {
	var newHead *ListNode = nil
	cur := head

	for cur != nil {
		t := cur.Next
		cur.Next = newHead
		newHead = cur
		cur = t
	}

	return newHead
}
