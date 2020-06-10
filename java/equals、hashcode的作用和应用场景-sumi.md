##equals:
确切来说是equals()方法，属于java.lang.Object类。 
'''
String aa="2222";  
String bb="3333";  
boolean cc=aa.equals(bb);
'''

对于比较两个字符串是否相等，有两种方法：使用“==”或者equals()。  
“==”比较两个变量本身的值，即两个对象在内存中的首地址。  
（**PS**：因为在Java中，对象的首地址是它在内存中存放的起始地址，它后面的地址是用来存放它所包含的各个属性的地址，所以内存中会用多个内存块来存放对象的各个参数，而通过这个首地址就可以找到该对象，进而可以找到该对象的各个属性）  
“equals()”比较的是两个字符串包含的内容是否相同，相同返回true，不同返回false。    
'''String s1,s2,s3 = "abc", s4 ="abc" ;  
s1 = new String("abc");  
s2 = new String("abc");  
s1==s2   是 false      //两个变量的内存地址不一样，也就是说它们指向的对象不 一样，  
s1.equals(s2) 是 true    //两个变量的所包含的内容是abc，故相等。'''  
==>“==”比较的是对象，而equals()比较的是内容。
---
但，在object类中定义的equals()方法其实也是比较对象的，即比较地址，但是由于有些类重新定义了equals()方法，所以用来比较内容。例如，String类。  
对于上述s3==s4的结果是true，因为s3、s4是两个由字符串常量生成的变量，该常量存放的内存地址不变，因此s3==s4的结果是true。  

总的来说，
equals方法对于字符串来说是比较内容的，而对于非字符串来说是比较，其指向的对象是否相同的。  
  == 比较符也是比较指向的对象是否相同的也就是对象在对内存中的的首地址。  
因此，如果是基本类型比较，那么只能用==来比较，且比较的是值，不能用equals，  
对于基本类型的包装类型，比如Boolean、Character、Byte、Shot、Integer、Long、Float、Double等的引用变量，==是比较地址的，而equals是比较内容的。  
注意：对于String(字符串)<比较内容>、StringBuffer(线程安全的可变字符序列)<比较地址>、StringBuilder(可变字符序列)这三个类作进一步的说明，若该类覆盖了equals方法，则具体怎么比较看重新定义，否则直接继承object的equals，比较地址。（下面给出一个例子）  
'''class Value  
{  
    int i;  
}  
public class LinkedListexample {  
    public static void main(String[] args) {  
        Value v1 = new Value();  
        Value v2 = new Value();  
        v1.i = v2.i = 100;  
        System.out.println(v1.equals(v2));//（1）flase  
        System.out.println(v1 == v2);//（2）flase  
    }  
}'''  
---

##hashcode：
hashCode()方法是给对象返回一个hash code值，Object类中定义的hashCode()方法对于不同对象会返回不同的Integer，这就是不同对象的hash code。  
在object类中，定义如下：  
'public native int hashCode(); '  
说明是一个本地方法，它的实现是根据本地机器相关的。  
但是，类似与equals方法一样，我们可以在自己的类中覆盖hashCode()方法。  
---
想要弄明白hashCode的作用，必须要先知道Java中的集合。  　　
       总的来说，Java中的集合（Collection）有两类，一类是List，再有一类是Set。前者集合内的元素是有序的，元素可以重复；后者元素无序，但元素不可重复。这里就引出一个问题：要想保证元素不重复，可两个元素是否重复应该依据什么来判断呢？  
        这就是Object.equals方法了。但是，如果每增加一个元素就检查一次，那么当元素很多时，后添加到集合中的元素比较的次数就非常多了。也就是说，如果集合中现在已经有1000个元素，那么第1001个元素加入集合时，它就要调用1000次equals方法。这显然会大大降低效率。     
       于是，Java采用了哈希表的原理。哈希（Hash）实际上是个人名，由于他提出一哈希算法的概念，所以就以他的名字命名了。哈希算法也称为散列算法，是将数据依特定算法直接指定到一个地址上，初学者可以简单理解，hashCode方法实际上返回的就是对象存储的物理地址（实际可能并不是）。    
       这样一来，当集合要添加新的元素时，先调用这个元素的hashCode方法，就一下子能定位到它应该放置的物理位置上。如果这个位置上没有元素，它就可以直接存储在这个位置上，不用再进行任何比较了；如果这个位置上已经有元素了，就调用它的equals方法与新元素进行比较，相同的话就不存了，不相同就散列其它的地址。所以这里存在一个冲突解决的问题。这样一来实际调用equals方法的次数就大大降低了，几乎只需要一两次。    
---
#####Java对象的eqauls方法和hashCode方法是这样规定的：
1.如果两个对象相同，那么它们的hashCode值一定要相同；  
 2.如果两个对象的hashCode相同，它们并不一定相同（这里说的对象相同指的是用eqauls方法比较）。    
3.equals()相等的两个对象，hashcode()一定相等；equals()不相等的两个对象，却并不能证明他们的hashcode()不相等。  
换句话说，equals()方法不相等的两个对象，hashcode()有可能相等（我的理解是由于哈希码在生成的时候产生冲突造成的）。反过来，hashcode()不等，一定能推出equals()也不等；hashcode()相等，equals()可能相等，也可能不等。    


##equals()与hashCode():
1.若重写了equals(Object obj)方法，则有必要重写hashCode()方法。  
2.若两个对象equals(Object obj)返回true，则hashCode（）有必要也返回相同的int数。  
3.若两个对象equals(Object obj)返回false，则hashCode（）不一定返回不同的int数。  
4.若两个对象hashCode（）返回相同int数，则equals（Object obj）不一定返回true。  
5.若两个对象hashCode（）返回不同int数，则equals（Object obj）一定返回false。    
6.同一对象在执行期间若已经存储在集合中，则不能修改影响hashCode值的相关信息，否则会导致内存泄露问题。  
这样的话，我们需要考虑，我们的类是否会被用于散列表本质的数据结构中，若不会，则不会产生该类的对应散列表，这种情况，我们比较两个类对象不需要考虑到hashCode方法，只需要考虑equals。  
但若存在该类的散列表，比较类对象，hashCode与equals是存在关系的，关系如上述所示。    