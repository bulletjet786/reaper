## 简单动态字符串

### 数据结构
```
struct __attribute__ ((__packed__)) sdshdr5 {
    unsigned char flags; /* 3 lsb of type, and 5 msb of string length */
    char buf[];
};
struct __attribute__ ((__packed__)) sdshdr8 {
    uint8_t len; /* used */
    uint8_t alloc; /* excluding the header and null terminator */
    unsigned char flags; /* 3 lsb of type, 5 unused bits */
    char buf[];
};
struct __attribute__ ((__packed__)) sdshdr16 {
    uint16_t len; /* used */
    uint16_t alloc; /* excluding the header and null terminator */
    unsigned char flags; /* 3 lsb of type, 5 unused bits */
    char buf[];
};
struct __attribute__ ((__packed__)) sdshdr32 {
    uint32_t len; /* used */
    uint32_t alloc; /* excluding the header and null terminator */
    unsigned char flags; /* 3 lsb of type, 5 unused bits */
    char buf[];
};
struct __attribute__ ((__packed__)) sdshdr64 {
    uint64_t len; /* used */
    uint64_t alloc; /* excluding the header and null terminator */
    unsigned char flags; /* 3 lsb of type, 5 unused bits */
    char buf[];
};
#define SDS_HDR_VAR(T,s) struct sdshdr##T *sh = (void*)((s)-(sizeof(struct sdshdr##T)));
#define SDS_HDR(T,s) ((struct sdshdr##T *)((s)-(sizeof(struct sdshdr##T))))
#define SDS_TYPE_5_LEN(f) ((f)>>SDS_TYPE_BITS)

// 一般情况下，结构体按其所有变量大小的最小公倍数字节对齐，用packet修饰后，结构体按1字节对齐。
```

### 头部
- flags: s[-1]
- 获取sds结构体：SDS_HDR(16,s)
- 获取sds结构体并赋值为sh变量：SDS_HDR_VAR(T,s)

### 字符串拼接扩容过程
1. 计算拼接后长度
2. 如无须扩容，则直接追加
3. 如需要扩容，则进行扩容，并计算拼接后数据类型
4. 如数据类型相同，则直接realloc
5. 如数据类型不同，则新建并迁移数据

### 扩容机制
1. 当len+addlen<1MB时，则新大小为2*(len+addlen)
2. 当len+addlen>1MB时，则新大小为len+addlen+1MB

## 整数集合

### 数据结构
```
typedef struct intset {
    uint32_t encoding;
    uint32_t length;
    int8_t contents[];
} intset;

编码类型：
INTSET_ENC_INT16：int16, 2个字节
INTSET_ENC_INT32: int32, 4个字节
INTSET_ENC_INT64: int64, 8个字节

intset按从小到大有序排序
```

### 查找元素
1. 如果要查找的元素>当前intset编码的最大值，则直接返回未找到
2. 检查是否>intset中的最大值或者<intset中的最小值，如果是，返回没找到
3. 二分查找该元素

### 添加元素
1. 如果待添加元素的编码大于当前intset的编码，则进行升级扩容并直接插入
2. 如果当前元素已存在，则返回
3. 如果当前元素不存在，则先扩容数组，大小+1
4. 数组插入元素
5. 长度+1

### 删除元素
1. 如果待删除元素的编码大于当前intset的编码，则进行返回
2. 查找到要删除元素的位置
3. 数组删除元素

## 跳表

### 数据结构


### 查找


### 插入


## 字典

### 数据结构
```
// 单链表，头插法
typedef struct dictEntry {
    void *key;
    union {
        void *val;
        uint64_t u64;
        int64_t s64;
        double d;
    } v;
    struct dictEntry *next;
} dictEntry;

// hash元素函数集，用于实现类似容器范型的功能
typedef struct dictType {
    uint64_t (*hashFunction)(const void *key);
    void *(*keyDup)(void *privdata, const void *key);
    void *(*valDup)(void *privdata, const void *obj);
    int (*keyCompare)(void *privdata, const void *key1, const void *key2);
    void (*keyDestructor)(void *privdata, void *key);
    void (*valDestructor)(void *privdata, void *obj);
} dictType;

// 哈希表，sizemask恒为size-1，size2次幂增长，加快运算
typedef struct dictht {
    dictEntry **table;
    unsigned long size;
    unsigned long sizemask;
    unsigned long used;
} dictht;

typedef struct dict {
    // 保存容器元素类型的特定函数集，相当于实现了范型
    dictType *type;
    // 保存容器元素类型相关的私有数据，配合type字段使用
    void *privdata;
    dictht ht[2];
    long rehashidx; /* rehash进度，-1表示未进行 */
    unsigned long iterators; /* 当前正在执行的安全迭代器个数 */
} dict;

// 安全迭代时，可以调用dictAdd, dictFind, 和其他函数；
// 非安全迭代时，只能调用dictNext()
typedef struct dictIterator {
    dict *d;
    long index;
    int table, safe;
    dictEntry *entry, *nextEntry;
    /* unsafe iterator fingerprint for misuse detection. */
    long long fingerprint;
} dictIterator;

// dictScan时对每个节点要调用的函数
typedef void (dictScanFunction)(void *privdata, const dictEntry *de);
// dictScan时整理碎片的函数
typedef void (dictScanBucketFunction)(void *privdata, dictEntry **bucketref);

// 哈希表初始大小
#define DICT_HT_INITIAL_SIZE     4
```

### 扩容

#### 扩容时机
- 初始大小为4
- 在每次调用_dictKeyIndex()查找key对应的index时，判断是否需要进行rehash
    - 扩容：dict_can_resize并使用量/空间>=1
    - 扩容：使用量/空间 > dict_force_resize_ratio(5)强制触发
    - 缩容：Resize()函数，由上游自己调用，在RedisObject-hashType层面，在t_hash.c中被调用，调用条件为使用量/空间不到10%
- 扩容大小：used的NextPower()

#### 渐进式Rehash
- 在添加/查询/删除等操作中，进行一次单步rehash，迁移一个桶，迁移完成会删除原桶
- 在DB层面，当服务器空闲时，将会进行批量迁移，每次对100个桶进行迁移，共执行1ms

### 迭代器
用于Redis内对字典进行迭代

#### 普通迭代器
- 用于sort等命令
- 只能使用dictNext函数
- 在开始迭代前和结束迭代后会对fingerprint进行断言验证

#### 安全迭代器
- 用于keys等命令
- 每生成一个会将绑定的dict的iterators加1，在iterators!=0，无法进行rehash，从而保证安全迭代器的正确
- nextEntry用于保证在安全迭代器中删除了当前entry节点后可以继续迭代后续数据

### Scan原理

#### cursor生成；高位加法算法
每次是高位加1的，也就是从左往右相加、从高到低进位的，其和扩容rehash桶成同模，利用这个特性，从而完成无遗漏，少重复的无状态遍历。

#### 扩容过程

- 在两次迭代中间完成了扩容
```
h[0]: 00 			[]	10 				01 				11 				00
h[0]: 000 	100 	[]	010 	110		001		101 	011		111		000
事件时序：客户端传来cursor=00 -> 扩容开始 -> 扩容完成 -> 客户端传来cursor=10
客户端收到的cursor时序：10 -> 110 -> ....
迭代数据：00 -> 010 -> 110 -> ....
从00->010是切换到扩容后的表的过程
重复数据：无
遗漏数据：无
```

- 在两次迭代中间完成了缩容
```
h[0]: 000 	100 	010 	[]	110		001		101 	011		111		000
h[0]: 		00 				[]	10 				01 				11 		00
事件时序：客户端传来000 -> 100 -> 客户端传来cursor=010 -> 缩容开始 -> 缩容完成 -> 客户端传来cursor=110
客户端收到的cursor时序：100 -> 010 -> 110 -> 01 ......
迭代数据：000 -> 100 -> 010 -> 10 -> 01 -> ...
从010->10是切换到缩容后的表的过程
重复数据：010
遗漏数据：无
```

- 在迭代过程中进行了扩容
```
h[0]: 00 			[	10 				01 				]	011 	111		000
h[1]: 000 	100 	[	010 	110		001		101 	]	011		111		000
事件时序：客户端传来cursor=00 -> 扩容开始 -> 客户端传来cursor=10 -> 客户端传来cursor=10 ->扩容完成
客户端收到的cursor时序：10 -> 01 -> 11 -> 00
迭代数据：00 -> 10+010+110 -> 01+001+101 -> 011 -> 111 
重复数据：无
遗漏数据：无
注意：
在rehash过程中，双表共存，则cursor以小表为准
如果10中有数据，数据还未对10进行rehsh，则010和110中就不会有数据，因为此时尚未对10迁移完成
如果010中有数据，则110中也必有数据，10中必然无数据，因为此时已经对10迁移完成
```

- 在迭代过程中进行了缩容
```
h[0]: 000 	100 	010 	[	110		001		101 	011		]	11		00
h[1]:		00 				[	10 				01 				]	11 		00
事件时序：客户端传来cursor=010 -> 扩容开始 -> 客户端传来cursor=110 -> 客户端传来cursor=10 ->扩容完成
客户端收到的cursor时序：100 -> 010 -> 110 -> 01 -> 00
迭代数据：000 -> 100 -> 010 -> 010+110+10 -> 001+101+01 -> 011+11 
从00->010是切换到扩容后的表的过程
重复数据：010
遗漏数据：无
注意：
在rehash过程中，双表共存，则cursor以小表为准
当客户端传来cursor=010迭代完010后，返回110，然后进行了缩容
当端上再次传来110时，该数据已迁移到了10中，原010和110中无数据，将会迭代10，包含了重复数据010
```

#### 优点
- 无状态：服务器不需要保存额外的状态
- 无遗漏

#### 缺点
- 可能重复：应用层可以解决
- count参数只作为给redis的提示，redis最少以桶为单位进行返回
- 高位加法的原理不易懂


## 跳表

### 数据结构


### 查找


### 插入