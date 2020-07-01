package sword

// 在一个 n * m 的二维数组中，每一行都按照从左到右递增的顺序排序，每一列都按照从上到下递增的顺序排序。
// 请完成一个函数，输入这样的一个二维数组和一个整数，判断数组中是否含有该整数。

// 最优解
// 时间复杂度：O(m+n)
// 空间复杂度：O(1)
// 原理：二分搜索
func findNumberIn2DArray(matrix [][]int, target int) bool {
	// nil处理
	if matrix == nil || len(matrix) == 0 || len(matrix[0]) == 0 {
		return false
	}

	row := 0
	col := len(matrix[0]) - 1
	for row < len(matrix) && col >= 0 {
		if matrix[row][col] == target {
			return true
		} else if matrix[row][col] > target {
			col--
		} else {
			row++
		}
	}
	return false
}

// 时间复杂度：O(m+n)
// 空间复杂度：O(1)
// 原理：遍历+二分
func findNumberIn2DArray2(matrix [][]int, target int) bool {
	// nil处理
	if matrix == nil || len(matrix) == 0 || len(matrix[0]) == 0 {
		return false
	}

	for i := 0; i < len(matrix); i++ {
		insertPosition := lowerBound(matrix[i], target)
		if insertPosition != len(matrix[i]) && matrix[i][insertPosition] == target {
			return true
		}
	}
	return false
}
