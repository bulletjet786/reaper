## 什么是clone
clone翻译过来就是克隆，简单来说就是复制。这里，有两个概念需要了解一下，就是浅拷贝和深拷贝。  
#### 浅拷贝：
浅拷贝是指我们拷贝出来的对象内部的引用类型变量和原来对象内部引用类型变量是同一引用（指向同一对象）。
但是我们拷贝出来的对象和新对象不是同一对象。  
简单来说就是，拷贝产生的新对象和原来的元对象不同，但是如果对象内部如果有引用类型的变量，新、旧对象引用的是同一引用。  
#### 深拷贝：
深拷贝是指我们拷贝原对象的全部内容，包括其内存的引用类型。  
#### java中的clone  
- 首先，clone()是Object类中的一个方法，并且是用protected关键字修饰的本地方法（native关键字修饰）。
完成克隆需要重写该方法。  
**注意：**重写的时候，需要将protected修饰符修改成public。
- 重写clone()方法的时候，要实现一个Cloneable接口。如果不实现就
会抛出异常：CloneNotSupportedException
- object中本地的clone()方法，默认是浅拷贝。  
![浅复制 图片](https://imgchr.com/i/tjq2Yn)
## 为什么clone是protected修饰
首先引用一下core java中的一段话，  
之所以把Object类中的clone方法定义为protected,是因为若把clone方法定义为public时,失去了安全机制.
这样的clone方法会被子类继承,而不管它对于子类有没有意义.  
比如,我们已经为Employee类定义了clone方法,而其他人可能会去克隆它的子类Manager对象.Employee克隆方法
能完成这件事吗?这取决于Manager类中的字段类型.如果Manager的实例字段是基本类型,不会发生什么问题.
但通常情况下,一需要检查你所扩展的任何类的clone方法.  
```
public class User implements Cloneable {
	public static void main(String[] args) {	
	User u =new User();
		try {
			u.clone();   //编译通过
		} catch (CloneNotSupportedException e) {
			e.printStackTrace();
		}
}
}

public class Test {
	public static void main(String[] args) {
		User u =new User();
		u.clone(); //编译错误，报错信息The method clone() from the type Object is not visible
	}
}
```   
Test类无法调用User类从父类Object继承来的clone()方法。User类自身可以调用。  
```
public class User implements Cloneable {
   
	@Override
	protected Object clone() throws CloneNotSupportedException {
		// TODO Auto-generated method stub
		return super.clone();
	}
	public static void main(String[] args) {
		User u =new User();
		try {
			u.clone();
		} catch (CloneNotSupportedException e) {
			// TODO Auto-generated catch block
			e.printStackTrace();
		}
}
}

public class Test {

	public static void main(String[] args) {
		User u =new User();
		try {
			u.clone();
		} catch (CloneNotSupportedException e) {
			// TODO Auto-generated catch block
			e.printStackTrace();
		}
	}

}
```
1. Object类中clone()方法声明为protected是一种保护机制，他的目的是在类中未重写Object的clone（）
方法的情况下，只能在本类里才能“克隆”本类的对象。  
2. 如果子类（比如User类）没有重写父类（Object类）受保护的clone()方法,它使用的就是从父类Object继承来的
clone()方法，Test类即使新建了user类的对象，但由于Test类与Object类是不同包的关系,所以无法调用user类的
clone方法。如果子类User重写了clone方法,调用的就是子类中clone方法,子类User与Test在同一个包中,
所以可以调用。