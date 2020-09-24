package sword

// 我们可以借鉴快速排序的思想。我们知道快排的划分函数每次执行完后都能将数组分成两个部分，小于等于分界值 pivot 的元素的都会被放到数组的左边，
// 大于的都会被放到数组的右边，然后返回分界值的下标。与快速排序不同的是，快速排序会根据分界值的下标递归处理划分的两侧，而这里我们只处理划分的一边。
// 我们定义函数 partition(arr, l, r, k) 表示划分数组 arr 的 [l,r] 部分，使前 k 小的数在数组的左侧，
// 在函数里我们调用快排的划分函数，假设划分函数返回的下标是 pos（表示分界值 pivot 最终在数组中的位置），
// 即 pivot 是数组中第 pos - l + 1 小的数，那么一共会有三种情况：
// 如果 pos - l + 1 == k，表示 pivot 就是第 kk 小的数，直接返回即可；
// 如果 pos - l + 1 < k，表示第 kk 小的数在 pivot 的右侧，因此递归调用 randomized_selected(arr, pos + 1, r, k - (pos - l + 1))；
// 如果 pos - l + 1 > k，表示第 kk 小的数在 pivot 的左侧，递归调用 randomized_selected(arr, l, pos - 1, k)。
func getLeastNumbers(arr []int, k int) []int {
	if k == 0 {
		return nil
	}
	if len(arr) <= k {
		return arr
	}

	start := 0
	end := len(arr) - 1
	m := partition(arr, start, end)
	for m != k-1 {
		if m > k-1 { // 左边的多于m
			end = m - 1
			m = partition(arr, start, end)
		} else { // 左边的少于m
			start = m + 1
			m = partition(arr, start, end)
		}
	}
	result := make([]int, 0)
	for i := 0; i < k; i++ {
		result = append(result, arr[i])
	}
	return result
}

func partition(a []int, start, end int) int {
	if start == end {
		return end
	}
	if start < end {
		pivot := a[start]
		l := start
		r := end
		for {
			for a[r] >= pivot && l < r {
				r--
			}
			for a[l] <= pivot && l < r {
				l++
			}
			if l >= r {
				break
			}
			a[l], a[r] = a[r], a[l]
		}
		a[start] = a[l]
		a[l] = pivot
		return l
	}
	return -1
}
