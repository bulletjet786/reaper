package sword

// 输入一个矩阵，按照从外向里以顺时针的顺序依次打印出每一个数字。
func spiralOrder(matrix [][]int) []int {
	if matrix == nil || len(matrix) == 0 || len(matrix[0]) == 0 {
		return nil
	}

	res := make([]int, 0)
	left := 0
	right := len(matrix[0]) - 1
	top := 0
	bottom := len(matrix) - 1

	i := 0
	j := 0
	for true {
		for j = left; j <= right; j++ {
			res = append(res, matrix[top][j])
		}
		top++
		if top > bottom {
			break
		}

		for i = top; i <= bottom; i++ {
			res = append(res, matrix[i][right])
		}
		right--
		if left > right {
			break
		}

		for j = right; j >= left; j-- {
			res = append(res, matrix[bottom][j])
		}
		bottom--
		if top > bottom {
			break
		}

		for i = bottom; i >= top; i-- {
			res = append(res, matrix[i][left])
		}
		left++
		if left > right {
			break
		}
	}

	return res
}
