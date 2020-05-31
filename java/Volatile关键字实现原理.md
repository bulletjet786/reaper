# Volatile关键字实现原理
在这一篇文章中，我将介绍java中实现volatile关键字相关的知识，包括编译屏障、内存屏障、JMM、内存一致性模型等。

我会先从volatile在java中的特性入手，然后讲解java中volatile在x86-tso中的实现，最后讲解java中volatile在JMM中是如何实现的，这样的安排个人觉得是由浅入深的，可以减少读者的阅读负担，使读者不需要一次性了解太多的概念。

这篇文章是我整理大量的文章后总结出来的，个人可以说服我自己，但是有错误是在所难免的，请大家不要尽信，多看其他资料进行验证。

## volatile变量特性
在java中，将一个变量声明为volatile，意味着该变量具有两个特性
- 可见性
- 有序性

对于这两个特性，从软件和硬件两次层面来进行解释，这里要强调一点的是，对于硬件工程师，关注的是可能更多的是硬件的实现，而对于软件工程师关注更多的则是硬件提供的抽象和承诺。理解这里有两个层次非常重要，对于软件层面，可以看作是接口，对于硬件层面，则应看作是实现。

### 可见性
在软件层面，意味着：对当前变量所做的修改在其他线程中是立即可见的

在硬件层面，意味着：每次对变量的读取都会从内存中获取而不从寄存器或者cache中获取缓存值，每次对变量的修改都会立即写入内存中

### 有序性

在软件层面，意味着：读写volatile变量之前的代码一定先于该行代码之后的代码执行

在硬件层面，意味着：利用编译屏障防止编译器指令重排序，利用内存屏障防止处理器和内存系统做指令重排序

#### 指令重排序
这些指令重排序的目的是编译器和处理器为了加快指令执行速度，从而尽可能优化指令排列，但是他们也带来了一些问题：编译器和处理器保证，对单线程程序，优化后的的执行顺序和优化前的执行顺序“看起来”一样，但是对于多线程程序，则不保证。

在讲述禁止指令重排序之前，我们先看一些指令重排序的例子：
```
a = 0
b = 0
c = 0
void p() {
    b = 1                   // 0
    a = 1                   // 1
    b = 2                   // 2
    c = a + b               // 3
}
void q() {
    while (true) {
        if (b == 2) {           // 4
            assert (a == 1)     // 5
        }
    }
}
```
1. 在单线程中，我们执行p()，指令1和指令2没有数据依赖关系，有可能会出现指令2在指令1之前执行，但是指令3一定是在指令1和指令2完成之后才执行，但是对于我们软件工程师来说，**在单线程中，我们观察不到重排序**，这是编译器和处理器提供给我们的承诺，是我们能够编写可预测代码的基础。

2. 现在来看一个多线程程序，我们分别执行在两个线程中执行p()和q()，有可能会出现断言失败的情形，按照我们的预测，当b为2的时候，一定有a为1，但是发生了重排序，指令2先于指令1执行了，导致断言失败了。注意**在多线程中，我们观察到了重排序**，这是编译器和处理器没有给我们提供的抽象。

---
编译器和处理器保证给软件工程师的承诺：

在单线程中，软件工程师观察不到重排序现象

在多线程中，软件工程师可以观察到重排序现象

---

在搞明白指令重排序之后，我们先看下都有哪些重排序操作：
- 编译器重排序：编译器（包括解释器）为了加快程序执行速度，会重新排序指令，排序后的指令在单线程中执行的结果会保证和原有的执行结果不变，但是在多线程时则不保证
- 指令重排序：CPU可以对没有数据依赖的指令乱序执行，在部分处理器，访存指令不会重排序，比如X86-TSO
- 内存伪重排序：由于在访问内存的中间包含有CPU独享级的缓存机制，使得在读写内存时看到不一致的值，就好像出现了指令乱序一样

#### 顺序一致性模型

因为在多线程中，线程之间的通信最终都会通过内存(哪怕是锁、信号量等同步机制)来完成，指令重排序在多线程编程中的影响也都是通过内存的访问次序来体现出来。

指令重排序为我们带来了多线程编程时的复杂性，但是考虑到指令重排序带来的对性能上的巨大提升和历史原因，短期内应该是不可能去掉指令重排序的，这是**现状**。

虽然指令重排序短期内无法去除，但我们可以建立一个模型帮助我们分析指令重排序带来的影响，在这个模型中，不会发生任何指令重排序，所有操作都是原子执行且立刻对其他线程可见，所有线程中的指令按照顺序依次执行，我们把这个模型叫做顺序一致性模型，这是**理想**。

---
顺序一致性模型：对于一个程序来说，所有操作都是原子执行且立刻对其他线程可见，在单线程程序，程序的执行顺序和从上到下依次执行指令的结果一致，在多线程程序中，指令的执行顺序是每个单线程执行顺序的交叉排列

---

>>> 在多线程程序中，指令的执行顺序是每个单线程执行顺序的交叉排列是指，想象你在使用两个打印线程各打印一个数组a1...a9和b1...b9，根据调度策略的不同你会得到不同的结果，但是顺序一致性保证你看到数组中，对于任意的a[n-1]都出现在a[n]前面。

在理想与现实之间，我们选择一个**权衡**，我们不要求完全的顺序一致性模型，这样性能太低，但是我们也不能任由指令重排序，这样对软件工程师的开发太过反直觉，我们允许指令重排序，但是我们要求编译器和处理器给我们提供一种机制，可以禁止某些指令重排序，从而实现局部性的顺序一致性，这个机制就是**内存屏障**，编译器提供给我们的叫做**优化屏障**，处理器提供给我们的叫做**CPU内存屏障**。

## X86-TSO

在讲解volatile之前，我们还需要了解一些X86-TSO的硬件知识
>>> TSO表示完全存储定序，是内存一致性模型的一种，内存一致性模型一共有四种，分别是SC-顺序一致模型（我们上面提到的顺序一致性模型），TSO-完全存储定序（我们即将要讲述的X86-TSO就是它的一个实现），PSO-部分存储定序，RMO-宽松存储定序，其内存一致性保证逐级降低

### X86-TSO的内存一致性模型

![X86-TSO内存模型](https://tva1.sinaimg.cn/large/007S8ZIlly1gf4mose05jj30l50av3yi.jpg)
- 在CPU级别，有寄存器，对于短时间频繁使用的变量，编译器可以选择将其值缓存在寄存器级别，在这期间，其他CPU看不到该变量内容的改变
- 在CPU和高速缓存之间有一个写缓冲区StoreBuffer，CPU会将写操作推入写缓冲区，在之后的某个时间点按照FIFO写入内存，在这期间，其他CPU看不到该变量内容的改变。注意，存在写缓冲区并且以FIFO进行工作，就是TSO-完全存储定序内存模型的工作方式。
- Cache，高速缓存，可以有多级，但是因为MESI缓存一致性协议可以保证各个处理器之间缓存一致，所以在这个层次上不会有各处理器数据不一致的情况
- 内存，各处理器共享，不会有数据不一致的情况

### X86-TSO内存模型可能出现的问题
```
// a、b为全局变量，r1、r2为局部变量
a = 0
b = 0
void cpu0() {
    store a = 1;     // Sa
    load r1 = b;     // Lb
}
void cpu1() {
    store b = 1;     // Sb
    load r2 = a      // La
}
```

#### SC-顺序一致性模型下的理想情况
按照SC-顺序一致性模型来说，该代码的执行次序有六种：

Sa Lb Sb La

Sa Sb Lb La

Sa Sb La Lb

Sb La Sa Lb

Sb Sa Lb La

Sb Sa La Lb

在这六种执行次序中，起码最后一条指令一定是个load指令，r1和r2中起码有一个值是1

#### TSO-完全存储定序模型下的异常情况
但是按照TSO模型，存在下面的情况，无法满足顺序一致性模型的要求：

指令Sa写入a为1到写缓冲区，加载b的值为0到r1局部变量；
指令Sb写入b为1到写缓冲区，加载a的值为0到r2局部变量中；
之后StoreBuffer的值被刷新到内存中，但此时为时已晚，r1=r2=0。

值得注意的是，这时候并没有发生指令重排序，但是因为StoreBuffer的存在，使得软件工程师“看起来”好像程序在执行过程把代码中的load指令重排序到了store前面了，这就是TSO模型下的StoreLoad重排序现象。

这里需要注意的是：实际上这是一个“可见性”的问题，由于StoreBuffer的存在，对一个变量的修改没有理解对其他线程中可见，

### 内存屏障
还记得吗？处理器为了提供性能放开了顺序一致性的严格限制，那么就必须提供某种给我们实现局部顺序一致性的机制，也就是X86内存屏障。那么X86给我们提供了哪些CPU内存屏障呢？

#### 编译器优化屏障
优化屏障可以用来禁止编译器在编译时进行指令重排序，如下就是一条优化屏障。
```
asm volatile("":::"memory")
```
**asm**: 表示该指令是条嵌入式汇编指令

**volatile**: 表示该指令不能被编译器优化

**"":::"memory"**: 这是gcc的嵌入式汇编格式，具体含义这里不讲，只讲我们需要的， 第一个""表示该指令是个空指令，上面的volatile保证这条空指令不被优化掉，第二个"memory"是破坏描述符，memory表示内存将会被修改，之后的指令不能使用寄存器中缓存的值，需要重新从内存中load进来，因为需要重新load，有内存依赖关系，所以后面的指令都不会重新排到前面

将第一个""里面的指令换成CPU内存屏障指令，就会使得原有指令变成一个同时具有CPU内存屏障和优化屏障的指令，这篇文章把这样同时具有运行时和编译时双重屏障功能的屏障叫做内存屏障指令。

---
GCC汇编指令中的"memory"描述符可以做到两件事：
- 让编译器后续指令的变量的值重新从内存中读取
- 让编译器不要把后面的指令不会重排到前面
 
从而实现优化屏障的功能

---

#### 单核
```
asm volatile("":::"memory")
```
在单核系统中，因为只有一个CPU，不存在多个CPU读写不同的存储，所以不会发生StoreLoad现象，直接使用空指令当作CPU内存屏障指令就行。

#### X86-32
```
asm volatile("lock; addl $0x0, (%esp)":::"memory")
```
在X86-32位处理器中，提供给我们一个lock指令前缀，lock前缀是一个特殊的信号，执行过程如下：

- 对总线和缓存上锁。
- 强制所有lock信号之前的指令，都在此之前被执行，并同步相关缓存。
- 执行lock后的指令（如cmpxchg）。
- 释放对总线和缓存上的锁。
- 强制所有lock信号之后的指令，都在此之后被执行，并同步相关缓存。

如上，lock指令可以实现内存屏障的功能，在上面的汇编语言中addl $0x0, (%esp)的含义是将栈顶加0，相当于空指令，这是因为lock语法不允许在后面写空指令，最后仍然是用memory实现优化屏障。

#### X86-64
```
asm volatile("mfence":::"memory")
asm volatile("lock; addl $0x0, (%esp)":::"memory")
```
在X86-64位处理器中，提供给我们一个mfence内存屏障指令，在mfence指令前的读写操作当必须在mfence指令后的读写操作前完成，最后仍然是用memory实现优化屏障。
这里值得注意的是，

---
在X86-TSO中，存在写缓冲器，从而可能出现StoreLoad重排序现象，为了解决这个问题，硬件开放给我们CPU内存屏障指令，加上编译器提供的优化屏障，软件工程师消除未正确同步的StoreLoad重排序现象，从而实现局部的顺序一致性。

---

## volatile语义在X86-TSO上的实现
```
a = 0
b = 0       // b是我们要实现的volatile变量
c = 0
void p() {
    a = 1
    // 插入编译器屏障，防止a=1重排到把b=2后面
    asm volatile("":::"memory")
    b = 2
    // 插入mfence指令，清空StoreBuffer，保证a=1的指令被刷新到内存中，也防止编译器将c=1重排到前面
    asm volatile("mfence":::"memory")
    c = 1
}
void q() {
    while (true) {
        c = 3
        // 插入编译器屏障，重新内存读取，防止c=3重排到后面
        asm volatile("":::"memory")
        r = b
        // 插入编译器屏障，重新内存读取，防止c=5重排到前面，if语句有数据依赖不会重排
        asm volatile("":::"memory")
        if (r == 2) {
            assert (a == 1)
        }
        c = 5
    }
}
```

## volatile在JMM上的实现
我们知道，java是跨平台的，java为所有平台提供一个统一的内存模型JMM，java需要在各种硬件内存模型上构建出JMM，我们接下来将会讨论怎么将volatile语义迁移到JMM上。

### 内存一致性模型
各种硬件众多，但是我们硬件内存一致性模型只有4种，就是我们上面提到的SC、TSO、PSO、RMO，所以JMM只需要在这四种内存一致性模型上实现voaltile的语义就可以了，而我们已经实现了TSO的volatile语义。我们上面了解到了X86-TSO的硬件细节，是为了我们更好的理解内存一致性问题，这里我们不再去探究其他硬件内存一致性模型的硬件细节，而是关注于不同内存一致性模型的抽象、问题和解决方案。

在访存指令上，Load指令用于读取内存，Store指令用于写入内存，按照操作形式，共有四种重排序现象：LoadLoad、LoadStore、StoreLoad、StoreStore。

X86-TSO有StoreLoad重排序现象，对此其提供了mfence和lock等内存屏障，其他内存一致性模型也有重排序现象，也提供了解决方案。

- SC：无重排序现象，也不需要解决方案
- TSO：有StoreLoad现象，需要可以实现StoreLoad内存屏障的指令
- PSO：有StoreStore/StoreLoad现象，需要可以实现StoreStore/StoreLoad内存屏障的指令
- RMO：有StoreStore/StoreLoad/LoadLoad/LoadStore现象，需要可以实现StoreStore/StoreLoad/LoadLoad/LoadStore内存屏障的指令

### 主流硬件的重排序现象和屏障指令
![主流](https://tva1.sinaimg.cn/large/007S8ZIlly1gfb06jducdj30q40tgaas.jpg)

注意：no-op是说不需要额外的CPU内存屏障指令，但是仍然需要编译器屏障指令

### JMM内存屏障
那么如何使用内存屏障实现volatile在JMM中的语义呢？Doug Lea大神在[The JSR-133 Cookbook for Compiler Writers](http://gee.cs.oswego.edu/dl/jmm/cookbook.html)文章中给出了答案，至于如何推导出这个答案，可以看[11]()，下面给出Doug Lea的答案：
![VolatileRules](https://tva1.sinaimg.cn/large/007S8ZIlly1gfb15kwwh7j30w20egmxo.jpg)
但是呢，这个规则对于编译器作者而言太不友好了，还要分第一操作和第二操作的类型，太复杂了，为此，编译器决定使用一个更加严格却更容易实现的规则来实现：
- 在每个volatile写前面加入一个StoreStore
- 在每个volatile写后面加入一个StoreLoad
- 在每个volatile读前面加入一个LoadLoad
- 在每个volatile读后面加入一个LoadStore

这也是我们上面在x86-TSO上所采用的规则。

接下来我们看看Java是如何实现这四种屏障的，以下是java12中的源码：
```
// 
static inline void compiler_barrier() {
  __asm__ volatile ("" : : : "memory");
}
inline void OrderAccess::fence() {
   // always use locked addl since mfence is sometimes expensive
#ifdef AMD64
  __asm__ volatile ("lock; addl $0,0(%%rsp)" : : : "cc", "memory");
#else
  __asm__ volatile ("lock; addl $0,0(%%esp)" : : : "cc", "memory");
#endif
  // 我觉得这句没有意义，但是多了也无伤大雅，因为编译器优化屏障并不会生成真正的指令，只是给编译器看的
  compiler_barrier();       
}
inline void OrderAccess::loadload()   { compiler_barrier(); }
inline void OrderAccess::storestore() { compiler_barrier(); }
inline void OrderAccess::loadstore()  { compiler_barrier(); }
inline void OrderAccess::storeload()  { fence();            }
```

### volatile实现机制
那么Java是如何使用这些内存屏障的呢？
```
// https://github.com/openjdk/jdk/blob/master/src/hotspot/share/interpreter/bytecodeInterpreter.cpp

// getstatic
TosState tos_type = cache->flag_state();
int field_offset = cache->f2_as_index();
if (cache->is_volatile()) {
  if (support_IRIW_for_not_multiple_copy_atomic_cpu) {
    OrderAccess::fence();
  }
  if (tos_type == atos) {
    VERIFY_OOP(obj->obj_field_acquire(field_offset));
    SET_STACK_OBJECT(obj->obj_field_acquire(field_offset), -1);
  } else if (tos_type == itos) {
    SET_STACK_INT(obj->int_field_acquire(field_offset), -1);
  } else if (tos_type == ltos) {
    ...
  }
} else {
  if (tos_type == atos) {
    VERIFY_OOP(obj->obj_field(field_offset));
    SET_STACK_OBJECT(obj->obj_field(field_offset), -1);
  } else if (tos_type == itos) {
    SET_STACK_INT(obj->int_field(field_offset), -1);
  } else if (tos_type == ltos) {
    ...    
  }
}

// putstatic
int field_offset = cache->f2_as_index();
if (cache->is_volatile()) {
  if (tos_type == itos) {
    obj->release_int_field_put(field_offset, STACK_INT(-1));
  } else if (tos_type == atos) {
    VERIFY_OOP(STACK_OBJECT(-1));
    obj->release_obj_field_put(field_offset, STACK_OBJECT(-1));
  } else if (tos_type == btos) {
    ...
  }
  OrderAccess::storeload();
} else {
  if (tos_type == itos) {
    obj->int_field_put(field_offset, STACK_INT(-1));
  } else if (tos_type == atos) {
    VERIFY_OOP(STACK_OBJECT(-1));
    obj->obj_field_put(field_offset, STACK_OBJECT(-1));
  } else if (tos_type == btos) {
    ...
  }
}

```
- 在读取volatile变量时，会调用int_field_acquire，而普通变量则会调用int_field
- 在写入volatile变量时，会调用release_int_field_put，在写入完成后会调用OrderAccess::storeload()内存屏障，而普通变量则会调用int_field_put
```
jint oopDesc::int_field_acquire(int offset) const                     { return HeapAccess<MO_ACQUIRE>::load_at(as_oop(), offset); }
void oopDesc::release_int_field_put(int offset, jint value)           { HeapAccess<MO_RELEASE>::store_at(as_oop(), offset, value); }
```



    
## 参考文档
- [Java内存模型](https://www.cnblogs.com/csniper/articles/5463138.html)
- [内存屏障与Volatile总结](http://www.chaozh.com/volatile-memory-barrior-summary/)
- [The JSR-133 Cookbook for Compiler Writers](http://gee.cs.oswego.edu/dl/jmm/cookbook.html)
- [编译时内存重排序](http://blog.sina.com.cn/s/blog_77e418120101m8b0.html)
- [内存一致性模型](http://www.wowotech.net/memory_management/456.html)
- [linux内核中的内存屏障](https://www.cnblogs.com/lysuns/articles/4771842.html)
- [CPU缓存一致性协议MESI](https://www.cnblogs.com/yanlong300/p/8986041.html)
- [面试必问的volatile，你了解多少](https://www.jianshu.com/p/506c1e38a922)
- [为什么X86架构下只有StoreLoad屏障是有效指令](https://zhuanlan.zhihu.com/p/81555436)
- [内存模型系列上-内存一致性模型](https://www.cnblogs.com/Kimbing-Ng/p/12829678.html)
- [從硬體觀點了解内存屏障的實作和效果](https://medium.com/fcamels-notes/%E5%BE%9E%E7%A1%AC%E9%AB%94%E8%A7%80%E9%BB%9E%E4%BA%86%E8%A7%A3-memry-barrier-%E7%9A%84%E5%AF%A6%E4%BD%9C%E5%92%8C%E6%95%88%E6%9E%9C-416ff0a64fc1)
- [Volatile与内存屏障总结](http://www.chaozh.com/volatile-memory-barrior-summary/)
- [JAVA虚拟机内存模型、指令重排、内存屏障概念解析](https://www.tuicool.com/wx/e2yy2ya)
- [指令重排序](https://uestc-dpz.github.io/blog/2016/11/17/Reordering.html)
- [intel x86系列CPU既然是strong-order的，不会出现loadload乱序，为什么还需要lfence指令？](https://www.zhihu.com/question/29465982)
- [Unsafe-Fence](https://www.it1352.com/542482.html)
- [C/C++Volatile关键词深度剖析](https://www.cnblogs.com/god-of-death/p/7852394.html)
- [内存屏障及其在JVM内的应用（下）](https://blog.csdn.net/EAPxUO/article/details/105851941)
- [优化屏障第一讲](https://blog.csdn.net/martin2350/article/details/8748347)
- [优化屏障第二讲](https://blog.csdn.net/martin2350/article/details/8748352)
- [优化屏障第三讲](https://blog.csdn.net/kissmonx/article/details/9334699)
- [Why Memory Barrier？](https://www.cnblogs.com/flintlovesam/p/7381132.html)
- [Volatile从入门到放弃](https://www.geek-share.com/detail/2695810486.html)