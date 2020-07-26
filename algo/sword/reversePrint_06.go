package sword

// 方法一：栈
//栈的特点是后进先出，即最后压入栈的元素最先弹出。考虑到栈的这一特点，使用栈将链表元素顺序倒置。
//从链表的头节点开始，依次将每个节点压入栈内，然后依次弹出栈内的元素并存储到数组中。

func reversePrint(head *ListNode) []int {
	var stack []int
	current := head
	for current != nil {
		stack = append(stack, current.Val)
		current = current.Next
	}
	var res []int
	for i := len(stack); i >= 0; i-- {
		res = append(res, stack[i])
	}
	return res
}
