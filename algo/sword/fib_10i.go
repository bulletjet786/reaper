package sword

// 写一个函数，输入 n ，求斐波那契（Fibonacci）数列的第 n 项。斐波那契数列的定义如下：
//
//F(0) = 0,   F(1) = 1
//F(N) = F(N - 1) + F(N - 2), 其中 N > 1.
//斐波那契数列由 0 和 1 开始，之后的斐波那契数就是由之前的两数相加而得出。
//
//答案需要取模 1e9+7（1000000007），如计算初始结果为：1000000008，请返回 1。

//动态规划解析：
//状态定义： 设 dpdp 为一维数组，其中 dp[i]dp[i] 的值代表 斐波那契数列第 ii 个数字 。
//转移方程： dp[i + 1] = dp[i] + dp[i - 1]dp[i+1]=dp[i]+dp[i−1] ，即对应数列定义 f(n + 1) = f(n) + f(n - 1)f(n+1)=f(n)+f(n−1) ；
//初始状态： dp[0] = 0dp[0]=0, dp[1] = 1dp[1]=1 ，即初始化前两个数字；
//返回值： dp[n]dp[n] ，即斐波那契数列的第 nn 个数字。
//

//求余运算规则： 设正整数 x, y, px,y,p ，则有 (x + y)%p = (x % p + y % p)
func fib(n int) int {
	if n == 0 {
		return 0
	}
	if n == 1 {
		return 1
	}
	if n == 2 {
		return 1
	}

	var (
		a   int
		b   int
		sum int
	)
	a = 1
	b = 1
	for i := 3; i <= n; i++ {
		sum = (a + b) % 1000000007
		a = b
		b = sum
	}
	return sum
}
