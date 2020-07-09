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

## 压缩列表

### 数据结构
redis的基本结构是一块字节数组，该字节数据的布局如下：
```
ziplist布局：
<zlbytes> <zltail> <zllen> <entry> <entry> ... <entry> <zlend>

<uint32_t zlbytes> ziplist占用字节数，可扩容
<uint32_t zltail> 指向最后一个entry，用于支持类似pop的操作
<uint16_t zllen> entry数量，最多为2^16-2，如果为2^16-1，则说明需要遍历ziplist获取长度
<uint8_t zlend> 最后entry，为特殊字符255
 
entry布局：
<prevlen> <encoding> <entry-data>

<prevlen> 前置entry长度，动态长度，有1或者5字节两种表示法，当首字节是0xFE为5字节表示法，用于后续遍历
<encoding> 该entry的编码和长度，动态长度，使用首字节部分位表示编码和长度
<entry-data> 该entry的数据

/* Return total bytes a ziplist is composed of. */

因为是一块字节数组而不是真正的结构体，故使用宏来获取各字段

/* 返回zlbytes */
#define ZIPLIST_BYTES(zl)       (*((uint32_t*)(zl)))

/* 返回zltail */
#define ZIPLIST_TAIL_OFFSET(zl) (*((uint32_t*)((zl)+sizeof(uint32_t))))

/* 返回长度zllen  */
#define ZIPLIST_LENGTH(zl)      (*((uint16_t*)((zl)+sizeof(uint32_t)*2)))

/* 返回头部大小 */
#define ZIPLIST_HEADER_SIZE     (sizeof(uint32_t)*2+sizeof(uint16_t))

/* 返回end entry大小 */
#define ZIPLIST_END_SIZE        (sizeof(uint8_t))

/* 返回head entry */
#define ZIPLIST_ENTRY_HEAD(zl)  ((zl)+ZIPLIST_HEADER_SIZE)

/* 返回tail entry */
#define ZIPLIST_ENTRY_TAIL(zl)  ((zl)+intrev32ifbe(ZIPLIST_TAIL_OFFSET(zl)))

/* 返回最后entry：zlend */
#define ZIPLIST_ENTRY_END(zl)   ((zl)+intrev32ifbe(ZIPLIST_BYTES(zl))-1)

```

辅助数据结构：因redis的entry表示解析复杂，使用zlentry辅助解析，不是真正的数据格式
```
/* 我们使用这个函数来接收一个ziplist的entry
 * 注意：这并不是数据的实际序列化格式，我们只是通过填充为这个结构体来简化操作。 */
typedef struct zlentry {
    unsigned int prevrawlensize; /* 用来保存前置节点长度字段的字节数 */
    unsigned int prevrawlen;     /* 前置节点长度 */
    unsigned int lensize;        /* 用来编码当前节点的字节数。字符串会有1，2，5三种字节情况，整数只会使用一个字节 */
    /* 当前节点长度。
     * 对于字符串，表示字符串长度，
     * 对于整数，可能为1，2，3，4，8或者0，0表示是立即数
     * */
    unsigned int len;
    unsigned int headersize;     /* 首部长度：前置节点长度和当前节点长度所占用字节数 */
    unsigned char encoding;      /* 编码类型，对于4位立即数，假设其在[0,12]之间 */
    unsigned char *p;            /* 指向ziplist entry的起始位置，即首部previous_entry_length */
} zlentry;
```


### zlentry的解码
```
void zipEntry(unsigned char *p, zlentry *e) {
    ZIP_DECODE_PREVLEN(p, e->prevrawlensize, e->prevrawlen);
    ZIP_DECODE_LENGTH(p + e->prevrawlensize, e->encoding, e->lensize, e->len);
    e->headersize = e->prevrawlensize + e->lensize;
    e->p = p;
}

// 解码prevlen字段
#define ZIP_DECODE_PREVLEN(ptr, prevlensize, prevlen) do
    ZIP_DECODE_PREVLENSIZE(ptr, prevlensize);
    if ((prevlensize) == 1) {
        // 1字节表示：长度为该字节表示的整数
        (prevlen) = (ptr)[0];
    } else if ((prevlensize) == 5) {
        // 5字节表示：长度为后四个字节表示的整数
        assert(sizeof((prevlen)) == 4);
        memcpy(&(prevlen), ((char*)(ptr)) + 1, 4);
        memrev32ifbe(&prevlen);
    }
} while(0);

/* 解码prevlen字段 */
#define ZIP_DECODE_PREVLENSIZE(ptr, prevlensize) do {
    if ((ptr)[0] < ZIP_BIG_PREVLEN) {
        // 1字节表示
        (prevlensize) = 1;
    } else {
        // 5字节表示 
        (prevlensize) = 5;
    }
} while(0);

/* 解码encoding和entry_length字段 */
#define ZIP_DECODE_LENGTH(ptr, encoding, lensize, len) do {
    ZIP_ENTRY_ENCODING((ptr), (encoding));
    if ((encoding) < ZIP_STR_MASK) {    // 字符串编码
        if ((encoding) == ZIP_STR_06B) {
            (lensize) = 1;
            (len) = (ptr)[0] & 0x3f;
        } else if ((encoding) == ZIP_STR_14B) {
            (lensize) = 2;
            (len) = (((ptr)[0] & 0x3f) < < 8) | (ptr)[1];
        } else if ((encoding) == ZIP_STR_32B) {
            (lensize) = 5;
            (len) = ((ptr)[1] < < 24) |
                    ((ptr)[2] < < 16) |
                    ((ptr)[3] < <  8) |
                    ((ptr)[4]);
        } else {
            panic(Invalid string encoding 0x%02X, (encoding));
        }
    } else {    // 整数编码
        (lensize) = 1;
        (len) = zipIntSize(encoding);
    }
} while(0);

/* 如果高两位是00，10，11，即小于 11000000，则为字符串编码 */
#define ZIP_ENTRY_ENCODING(ptr, encoding) do {
    (encoding) = (ptr[0]);
    if ((encoding) < ZIP_STR_MASK) (encoding) &= ZIP_STR_MASK;
} while(0)
```
总结：
- 使用字节数组扁平存储，使用宏来获取相关字段
- 动态长度编码，使用特殊位或者特殊值表示编码和长度，目的是为了节省内存

### 插入
1. 编码：prevlen字段、encoding字段、content字段
    1. 计算prevlen字段：
        - 如果插入头部，则prevlen=0；
        - 如果插入中间，则prevlen=当前待插入位置节点的prevlen
        - 如果插入尾部，则找到tail节点，直接计算长度
    2. 计算encoding字段：尝试将数据内容解析为整数，若成功，则按整数编码，若失败，则按字符串编码
    3. 计算content字段：按照encoding进行编码
2. 计算新entry的长度：reqlen
3. 扩容ziplist：reqlen+nextdiff
    - nextdiff是后继节点的prevlen字段长度变化
    - 如果nextdiff+reqlen<0，则使用5字节编码prevlen以保证不会缩容
4. 插入新entry，并迁移所有后继节点
```
/* 将s插入到ziplist的p位置 */
unsigned char *__ziplistInsert(unsigned char *zl, unsigned char *p, unsigned char *s, unsigned int slen) {
    size_t curlen = intrev32ifbe(ZIPLIST_BYTES(zl)), reqlen;
    unsigned int prevlensize, prevlen = 0;
    size_t offset;
    int nextdiff = 0;
    unsigned char encoding = 0;
    long long value = 123456789; /* initialized to avoid warning. Using a value
                                    that is easy to see if for some reason
                                    we use it uninitialized. */
    zlentry tail;

    /* 计算待插入节点的前置长度 */
    if (p[0] != ZIP_END) {
        // 如果插入到中间位置，则取当前待插入位置节点的prevlen字段作为prevlen
        ZIP_DECODE_PREVLEN(p, prevlensize, prevlen);
    } else {
        // 如果插入到结尾位置，则通过tail找到最后一个节点
        unsigned char *ptail = ZIPLIST_ENTRY_TAIL(zl);
        if (ptail[0] != ZIP_END) {
            // 如果最后一个节点不是终止节点，则直接计算最后一个节点的长度
            prevlen = zipRawEntryLength(ptail);
        }
    }

    /* 观察是否可以被解析成整数，然后：reqlen=entry->length所需空间 */
    if (zipTryEncoding(s,slen,&value,&encoding)) {
        /* 如果可以，那么通过encoding计算出entry->length */
        reqlen = zipIntSize(encoding);
    } else {
        /* 如果不能，则直接使用元素s的长度作为entry->length，encoding字段将会在zipStoreEntryEncoding中进行解析 */
        reqlen = slen;
    }
    /* 计算prevlen所需空间和content所需空间，此时reqlen=space(entry) */
    reqlen += zipStorePrevEntryLength(NULL,prevlen);
    reqlen += zipStoreEntryEncoding(NULL,encoding,slen);

    /* When the insert position is not equal to the tail, we need to
     * make sure that the next entry can hold this entry's length in
     * its prevlen field. */
    /* 当插入的位置不是结尾时，我们需要确保下一个entry可以持有前置节点的长度 */
    int forcelarge = 0;
    nextdiff = (p[0] != ZIP_END) ? zipPrevLenByteDiff(p,reqlen) : 0;
    // 如果出现了下一节点的prevlen长度从5字节变为1字节的情况，并且新节点需要的空间少于4，则会
    // 出现插入节点反而使得ziplist总长度更小的情况，此时realloc函数可能将会直接收缩
    // 此时，对于prevlen字段，我们仍然使用5字节格式进行保存，并强制ziplist增长
    if (nextdiff == -4 && reqlen < 4) {
        nextdiff = 0;
        forcelarge = 1;
    }

    /* Store offset because a realloc may change the address of zl. */
    /* 保存offset因为realloc可能会修改zl的地址 */
    offset = p-zl;
    zl = ziplistResize(zl,curlen+reqlen+nextdiff);
    p = zl+offset;

    /* 当插入entry不是end时，需要迁移内存，并更新tail */
    if (p[0] != ZIP_END) {
        /* Subtract one because of the ZIP_END bytes */
        memmove(p+reqlen,p-nextdiff,curlen-offset-1+nextdiff);

        /* Encode this entry's raw length in the next entry. */
        /* 在下一个节点写入prevlen，对于forcelarge，强制使用5字节保存 */
        if (forcelarge)
            zipStorePrevEntryLengthLarge(p+reqlen,reqlen);
        else
            zipStorePrevEntryLength(p+reqlen,reqlen);

        /* 更新tail */
        ZIPLIST_TAIL_OFFSET(zl) =
            intrev32ifbe(intrev32ifbe(ZIPLIST_TAIL_OFFSET(zl))+reqlen);

        /* When the tail contains more than one entry, we need to take
         * "nextdiff" in account as well. Otherwise, a change in the
         * size of prevlen doesn't have an effect on the *tail* offset. */
        /* 后继节点的prevlen字节长度有可能改变，tail需要加上nextdiff */
        /* 如果p+reqlen后续包含多个节点，nextdiff导致tail偏移量不再指向结尾元素，则需要修复tail偏移量 */
        zipEntry(p+reqlen, &tail);
        if (p[reqlen+tail.headersize+tail.len] != ZIP_END) {
            ZIPLIST_TAIL_OFFSET(zl) =
                intrev32ifbe(intrev32ifbe(ZIPLIST_TAIL_OFFSET(zl))+nextdiff);
        }
    } else {
        /* 当插入entry是end时，只需要更新tail */
        ZIPLIST_TAIL_OFFSET(zl) = intrev32ifbe(p-zl);
    }

    /* When nextdiff != 0, the raw length of the next entry has changed, so
     * we need to cascade the update throughout the ziplist */
    /* 当nextdiff不为0的时候，nextEntry的长度可能会修改，我们需要进行级联更新 */
    if (nextdiff != 0) {
        offset = p-zl;
        zl = __ziplistCascadeUpdate(zl,p+reqlen);
        p = zl+offset;
    }

    /* 写入entry */
    p += zipStorePrevEntryLength(p,prevlen);
    p += zipStoreEntryEncoding(p,encoding,slen);
    if (ZIP_IS_STR(encoding)) {
        memcpy(p,s,slen);
    } else {
        zipSaveInteger(p,value,encoding);
    }
    ZIPLIST_INCR_LENGTH(zl,1);
    return zl;
}
```

### 删除
1. 计算待删除元素的总长度
2. 计算nextdiff
3. 从待删除的空间末尾，截取nextdiff空间，加上后继节点的prevlen，作为后继节点的新的prevlen
4. 迁移内存空间
5. 缩容ziplist
```
/* 删除从p开始的连续num个entry */
unsigned char *__ziplistDelete(unsigned char *zl, unsigned char *p, unsigned int num) {
    unsigned int i, totlen, deleted = 0;
    size_t offset;
    int nextdiff = 0;
    zlentry first, tail;

    zipEntry(p, &first);
    for (i = 0; p[0] != ZIP_END && i < num; i++) {
        p += zipRawEntryLength(p);
        deleted++;
    }

    totlen = p-first.p;     /* 要删除的字节长度 */
    if (totlen > 0) {
        if (p[0] != ZIP_END) {
            /* Storing `prevrawlen` in this entry may increase or decrease the
             * number of bytes required compare to the current `prevrawlen`.
             * There always is room to store this, because it was previously
             * stored by an entry that is now being deleted. */
            /* 计算p节点的长度字节差 */
            nextdiff = zipPrevLenByteDiff(p,first.prevrawlen);

            /* Note that there is always space when p jumps backward: if
             * the new previous entry is large, one of the deleted elements
             * had a 5 bytes prevlen header, so there is for sure at least
             * 5 bytes free and we need just 4. */
            /* p迁移，并保存新的节点 */
            p -= nextdiff;
            zipStorePrevEntryLength(p,first.prevrawlen);

            /* Update offset for tail */
            /* 更新tail */
            ZIPLIST_TAIL_OFFSET(zl) =
                intrev32ifbe(intrev32ifbe(ZIPLIST_TAIL_OFFSET(zl))-totlen);

            /* When the tail contains more than one entry, we need to take
             * "nextdiff" in account as well. Otherwise, a change in the
             * size of prevlen doesn't have an effect on the *tail* offset. */
            /* 后继节点的prevlen字节长度有可能改变，tail需要加上nextdiff */
            /* 如果p后续包含多个节点，nextdiff导致tail偏移量不再指向结尾元素，则需要修复tail偏移量 */
            zipEntry(p, &tail);
            if (p[tail.headersize+tail.len] != ZIP_END) {
                ZIPLIST_TAIL_OFFSET(zl) =
                   intrev32ifbe(intrev32ifbe(ZIPLIST_TAIL_OFFSET(zl))+nextdiff);
            }

            /* Move tail to the front of the ziplist */
            /* 迁移后面的节点到前面来 */
            memmove(first.p,p,
                intrev32ifbe(ZIPLIST_BYTES(zl))-(p-zl)-1);
        } else {
            /* The entire tail was deleted. No need to move memory. */
            /* tail节点被删除了，不需要一定内存，直接设置tail就可以了 */
            ZIPLIST_TAIL_OFFSET(zl) =
                intrev32ifbe((first.p-zl)-first.prevrawlen);
        }

        /* 缩容：Resize ziplist长度 */
        offset = first.p-zl;
        zl = ziplistResize(zl, intrev32ifbe(ZIPLIST_BYTES(zl))-totlen+nextdiff);
        ZIPLIST_INCR_LENGTH(zl,-deleted);
        p = zl+offset;

        /* When nextdiff != 0, the raw length of the next entry has changed, so
         * we need to cascade the update throughout the ziplist */
        /* 当nextdiff不为0时，可能会级联更新 */
        if (nextdiff != 0)
            zl = __ziplistCascadeUpdate(zl,p);
    }
    return zl;
}
```


### 遍历
```
/* 前向遍历 */
unsigned char *ziplistNext(unsigned char *zl, unsigned char *p) {
    ((void) zl);
     /* 在迭代的过程中可能出现删除操作，使得p有可能为ZIP_END， 此时返回NULL */
    if (p[0] == ZIP_END) {
        return NULL;
    }

    p += zipRawEntryLength(p);
    if (p[0] == ZIP_END) {
        return NULL;
    }

    return p;
}

/* 反向遍历 */
unsigned char *ziplistPrev(unsigned char *zl, unsigned char *p) {
    unsigned int prevlensize, prevlen = 0;

     /* 如果当前节点是ZIP_END，应该返回tail节点，如果当前节点是head，应该返回NULL */
    if (p[0] == ZIP_END) {
        p = ZIPLIST_ENTRY_TAIL(zl);
        return (p[0] == ZIP_END) ? NULL : p;
    } else if (p == ZIPLIST_ENTRY_HEAD(zl)) {
        return NULL;
    } else {
        ZIP_DECODE_PREVLEN(p, prevlensize, prevlen);
        assert(prevlen > 0);
        return p-prevlen;
    }
}
```

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
```
/* skiplist节点 */
typedef struct zskiplistNode {
    sds ele;
    double score;
    struct zskiplistNode *backward;
    struct zskiplistLevel {
        struct zskiplistNode *forward;
        unsigned long span;
    } level[];
} zskiplistNode;

typedef struct zskiplist {
    struct zskiplistNode *header, *tail;    // 首尾节点
    unsigned long length;                   // 长度
    int level;                              // 当前层高
} zskiplist;

/* 用于表达区间，[min,max]\(min,max)\[min,max)\(min,max] */
typedef struct {
    double min, max;
    int minex, maxex;  // min和max是否是开放的
} zrangespec;
```

### 创建
```
/* 创建一个skiplist节点，sds的所有权将会转移给siplist节点 */
zskiplistNode *zslCreateNode(int level, double score, sds ele) {
    zskiplistNode *zn =
        zmalloc(sizeof(*zn)+level*sizeof(struct zskiplistLevel));
    zn->score = score;
    zn->ele = ele;
    return zn;
}

/* 创建一个列表，设置header节点，header节点是个特殊节点，其层高为64但不计入level计算，span为0，且ele为空，且不会有任何节点指向该节点 */
zskiplist *zslCreate(void) {
    int j;
    zskiplist *zsl;

    zsl = zmalloc(sizeof(*zsl));
    zsl->level = 1;
    zsl->length = 0;
    zsl->header = zslCreateNode(ZSKIPLIST_MAXLEVEL,0,NULL);
    for (j = 0; j < ZSKIPLIST_MAXLEVEL; j++) {
        zsl->header->level[j].forward = NULL;
        zsl->header->level[j].span = 0;
    }
    zsl->header->backward = NULL;
    zsl->tail = NULL;
    return zsl;
}
```

### 插入
```
/* 插入一个节点。假设调用方保证在跳表中不存在该节点，该跳表会接手ele的所有权 */
zskiplistNode *zslInsert(zskiplist *zsl, double score, sds ele) {
    zskiplistNode *update[ZSKIPLIST_MAXLEVEL], *x;
    unsigned int rank[ZSKIPLIST_MAXLEVEL];
    int i, level;

    serverAssert(!isnan(score));
    x = zsl->header;

    // 查找待插入位置的所有前置节点：
    // 从最高层向右遍历，如果下一节点的sroce小于当前值，则向右，否则向下，直到下到最底层
    // 在每次向下时，记录当前节点
    for (i = zsl->level-1; i >= 0; i--) {
        /* store rank that is crossed to reach the insert position */
        /* rank表示当前层从header节点到update节点沿途共跨越了多少个节点到达插入点，用于计算span */
        rank[i] = i == (zsl->level-1) ? 0 : rank[i+1];
        while (x->level[i].forward &&
                (x->level[i].forward->score < score ||
                    (x->level[i].forward->score == score &&
                    sdscmp(x->level[i].forward->ele,ele) < 0)))
        {
            rank[i] += x->level[i].span;
            x = x->level[i].forward;
        }
        update[i] = x;
    }
    /* zslInsert()的调用方应该在hashtable中确保相同sds和score的节点不会被插入 */
    /* 计算随机高度，如果新节点高于最高节点，则需要设置header新扩展的高层节点 */
    level = zslRandomLevel();
    if (level > zsl->level) {
        for (i = zsl->level; i < level; i++) {
            rank[i] = 0;
            update[i] = zsl->header;
            update[i]->level[i].span = zsl->length;
        }
        zsl->level = level;
    }
    // 插入节点
    x = zslCreateNode(level,score,ele);
    for (i = 0; i < level; i++) {
        /* 插入节点 */
        x->level[i].forward = update[i]->level[i].forward;
        update[i]->level[i].forward = x;

        /* update span covered by update[i] as x is inserted here */
        /* 更新前置节点和当前节点的span */
        x->level[i].span = update[i]->level[i].span - (rank[0] - rank[i]);
        update[i]->level[i].span = (rank[0] - rank[i]) + 1;
    }

    /* increment span for untouched levels */
    /* 高于插入节点层数的所有update的span增加1 */
    for (i = level; i < zsl->level; i++) {
        update[i]->level[i].span++;
    }

    // 任何节点都不能指向header节点，first的节点应为NULL
    // 如果其前置节点是header节点，则意味着插入节点是first节点，则设置backward为NULL
    x->backward = (update[0] == zsl->header) ? NULL : update[0];
    // 如果插入节点存在后继节点，则更新后继节点的backward，否则更新跳表的tail
    if (x->level[0].forward)
        x->level[0].forward->backward = x;
    else
        zsl->tail = x;
    zsl->length++;
    return x;
}

/* 获取一个随机层高，进入下一层的概率是P=0.25 */
int zslRandomLevel(void) {
    int level = 1;
    while ((random()&0xFFFF) < (ZSKIPLIST_P * 0xFFFF))
        level += 1;
    return (level<ZSKIPLIST_MAXLEVEL) ? level : ZSKIPLIST_MAXLEVEL;
}
```

### 删除：by ele+score
```
/* zslDelete，zslDeleteByScore，zslDeleteByRank 所使用的内部函数 */
void zslDeleteNode(zskiplist *zsl, zskiplistNode *x, zskiplistNode **update) {
    int i;
    // 调整update的forward和span
    for (i = 0; i < zsl->level; i++) {
        if (update[i]->level[i].forward == x) {
            update[i]->level[i].span += x->level[i].span - 1;
            update[i]->level[i].forward = x->level[i].forward;
        } else {
            update[i]->level[i].span -= 1;
        }
    }
    // 调整后置节点的backward指针和tail指针
    if (x->level[0].forward) {
        x->level[0].forward->backward = x->backward;
    } else {
        zsl->tail = x->backward;
    }
    // 删除后，层高可能会下降，从高层检查，直到出现header不指向NULL尾节点，则该层是新的最高层，调整skiplist的level
    while(zsl->level > 1 && zsl->header->level[zsl->level-1].forward == NULL)
        zsl->level--;
    zsl->length--;
}

/* 删除一个匹配sds和说score的节点
 *
 * 如果参数node为null，节点将会被删除，否则节点将会被设置到node参数中以便调用方重用
 * */
int zslDelete(zskiplist *zsl, double score, sds ele, zskiplistNode **node) {
    zskiplistNode *update[ZSKIPLIST_MAXLEVEL], *x;
    int i;

    x = zsl->header;
    for (i = zsl->level-1; i >= 0; i--) {
        while (x->level[i].forward &&
                (x->level[i].forward->score < score ||
                    (x->level[i].forward->score == score &&
                     sdscmp(x->level[i].forward->ele,ele) < 0)))
        {
            x = x->level[i].forward;
        }
        update[i] = x;
    }
    /* 可能有多个元素的score相同，我们需要检查是否找到了正确的对象 */
    x = x->level[0].forward;
    if (x && score == x->score && sdscmp(x->ele,ele) == 0) {
        zslDeleteNode(zsl, x, update);
        if (!node)
            zslFreeNode(x);
        else
            *node = x;
        return 1;
    }
    return 0; /* not found */
}
```

### 删除：by score range
```
/* 删除score位于[min,max]中间的所有节点， 这个函数也会从dict参数中删除对应的元素 */
unsigned long zslDeleteRangeByScore(zskiplist *zsl, zrangespec *range, dict *dict) {
    zskiplistNode *update[ZSKIPLIST_MAXLEVEL], *x;
    unsigned long removed = 0;
    int i;

    // 获取待删除区间第一个元素的update数组
    x = zsl->header;
    for (i = zsl->level-1; i >= 0; i--) {
        while (x->level[i].forward && (range->minex ?
            x->level[i].forward->score <= range->min :
            x->level[i].forward->score < range->min))
                x = x->level[i].forward;
        update[i] = x;
    }

    /* Current node is the last with score < or <= min. */
    /* 遍历完后，当前节点是不满足下界条件的最后一个节点，x的forward则是第一个满足下界条件的第一个节点 */
    x = x->level[0].forward;

    /* Delete nodes while in range. */
    /* 从当前节点开始一直删除，直到不满足上界条件 */
    while (x &&
           (range->maxex ? x->score < range->max : x->score <= range->max))
    {
        zskiplistNode *next = x->level[0].forward;
        zslDeleteNode(zsl,x,update);
        dictDelete(dict,x->ele);
        zslFreeNode(x); /* Here is where x->ele is actually released. */
        removed++;
        x = next;
    }
    return removed;
}
```

### 删除：by rank range
```
/* 删除[start,end]的所有节点 */
unsigned long zslDeleteRangeByRank(zskiplist *zsl, unsigned int start, unsigned int end, dict *dict) {
    zskiplistNode *update[ZSKIPLIST_MAXLEVEL], *x;
    unsigned long traversed = 0, removed = 0;
    int i;

    // 获取待删除区间第一个元素的update数组
    x = zsl->header;
    for (i = zsl->level-1; i >= 0; i--) {
        while (x->level[i].forward && (traversed + x->level[i].span) < start) {
            traversed += x->level[i].span;
            x = x->level[i].forward;
        }
        update[i] = x;
    }

    // 指向第一个待删除元素
    traversed++;
    x = x->level[0].forward;

    // 逐个删除[start,end]区间元素
    while (x && traversed <= end) {
        zskiplistNode *next = x->level[0].forward;
        zslDeleteNode(zsl,x,update);
        dictDelete(dict,x->ele);
        zslFreeNode(x);
        removed++;
        traversed++;
        x = next;
    }
    return removed;
}
```

## 快表

### 数据结构
```
typedef struct quicklistNode {
    struct quicklistNode *prev;
    struct quicklistNode *next;
    unsigned char *zl;
    unsigned int sz;             /* ziplist占用字节数 */
    unsigned int count : 16;     /* 所有ziplist的entry数量总和 */
    unsigned int encoding : 2;   /* RAW==1 or LZF==2 */
    unsigned int container : 2;  /* NONE==1 or ZIPLIST==2 */
    unsigned int recompress : 1; /* 这个节点之前是否被压缩过，用于临时解压缩 */
    unsigned int attempted_compress : 1; /* 用于调试 */
    unsigned int extra : 10; /* 将会可能会使用 */
} quicklistNode;

typedef struct quicklistLZF {
    unsigned int sz; /* LZF压缩数据字节数,=len(compressed)*/
    char compressed[];
} quicklistLZF;

typedef struct quicklist {
    quicklistNode *head;
    quicklistNode *tail;
    unsigned long count;        /* 所有ziplist的entry数量总和 */
    unsigned long len;          /* quicklistNode的节点数量 */
    /* 填充因子：针对于单个节点 */
    /* 当为正数时，表示每个ziplist的最多含有的entry数 */
    /* 当为负数时，表示每个ziplist的长度，-1=4K，-2=8K，-3=16K，-4=32K，-5=64K，在该值上下浮动 */
    int fill : 16;              /* 单个节点的填充因子 */
    /* quicklist两端不允许进行压缩的ziplist数，-1表示禁用 */
    unsigned int compress : 16; /* 压缩深度，list两边的depth个节点将不会被压缩，0表示禁用压缩 */
} quicklist;

typedef struct quicklistIter {
    const quicklist *quicklist; /* 当前正在迭代的quicklist */
    quicklistNode *current; /* 当前正在迭代的node */
    unsigned char *zi;  /* 当前正在迭代的ziplist的位置 */
    long offset; /* 当前正在迭代ziplist的第offset个元素 */
    int direction; /* 迭代方向 */
} quicklistIter;

/* 表示ziplist中的一个entry */
typedef struct quicklistEntry {
    const quicklist *quicklist;
    quicklistNode *node;
    unsigned char *zi;  /* 在所指向的ziplist的位置 */
    unsigned char *value; /* entry的字符串内容 */
    long long longval;  /* entry所表示的整形值 */
    unsigned int sz; /* entry的字符串内容长度 */
    int offset;     /* 是ziplist中第offset个entry */
} quicklistEntry;
```

### 插入: 头部
```
/* 从头部插入一个enrty，如果使用已经存在的头部节点则返回0，如果创建了一个新的节点则返回1 */
int quicklistPushHead(quicklist *quicklist, void *value, size_t sz) {
    quicklistNode *orig_head = quicklist->head;
    if (likely(
            _quicklistNodeAllowInsert(quicklist->head, quicklist->fill, sz))) {
        // 如果head节点允许插入，则直接进行在head上插入，然后更新头部节点的sz
        quicklist->head->zl =
            ziplistPush(quicklist->head->zl, value, sz, ZIPLIST_HEAD);
        quicklistNodeUpdateSz(quicklist->head);
    } else {
        // 否则需要创建一个新的节点，向其ziplist中插入数据，并更新node的sz，
        // 然后将该节点插入大quicklist首部
        quicklistNode *node = quicklistCreateNode();
        node->zl = ziplistPush(ziplistNew(), value, sz, ZIPLIST_HEAD);

        quicklistNodeUpdateSz(node);
        _quicklistInsertNodeBefore(quicklist, quicklist->head, node);
    }
    quicklist->count++;
    quicklist->head->count++;
    return (orig_head != quicklist->head);
}

/* quicklistNode是否允许插入 */
REDIS_STATIC int _quicklistNodeAllowInsert(const quicklistNode *node,
                                           const int fill, const size_t sz) {
    if (unlikely(!node))
        return 0;

    int ziplist_overhead;
    /* size of previous offset */
    /* 计算要增加的entry可能导致增加的最大prevlen长度占用字节数 */
    if (sz < 254)
        ziplist_overhead = 1;
    else
        ziplist_overhead = 5;

    /* size of forward offset */
    /* 要插入的entry的长度字段占用字节数 */
    if (sz < 64)
        ziplist_overhead += 1;
    else if (likely(sz < 16384))
        ziplist_overhead += 2;
    else
        ziplist_overhead += 5;

    /* new_sz overestimates if 'sz' encodes to an integer type */
    /* 如果sz可以被解析成一个integer，那个new_sz的估计是过高 */
    /* 这里不考虑解析成integer的场景，总是把要插入的元素当作str来处理 */
    unsigned int new_sz = node->sz + sz + ziplist_overhead;

    // 当fill表示size时，则 当size满足fill要求时允许插入
    // 当fill表示count时，则 当新entry小于等于8K且count满足fill要求时允许插入

    // 1. 如果fill<0且增长后的ziplist的size 小于 fill限定的最大size，则允许插入
    // 2. 原节点中已有entry，如果新插入的entry的节点size 大于 8K，则不允许插入
    // 3. 如果fill>0且count 小于 fill限定的最大count，则允许插入
    // 4. 否则不允许插入
    if (likely(_quicklistNodeSizeMeetsOptimizationRequirement(new_sz, fill)))
        return 1;
    else if (!sizeMeetsSafetyLimit(new_sz))
        return 0;
    else if ((int)node->count < fill)
        return 1;
    else
        return 0;
}

/* 更新quicklist的sz */
#define quicklistNodeUpdateSz(node)                                            \
    do {                                                                       \
        (node)->sz = ziplistBlobLen((node)->zl);                               \
    } while (0)

```

### 插入: 中间
```
REDIS_STATIC void _quicklistInsert(quicklist *quicklist, quicklistEntry *entry,
                                   void *value, const size_t sz, int after) {
    int full = 0, at_tail = 0, at_head = 0, full_next = 0, full_prev = 0;
    int fill = quicklist->fill;
    quicklistNode *node = entry->node;
    quicklistNode *new_node = NULL;

    if (!node) {
        /* 如果没给定quicklistNode，则创建一个新的节点插入到头部 */
        D("No node given!");
        new_node = quicklistCreateNode();
        new_node->zl = ziplistPush(ziplistNew(), value, sz, ZIPLIST_HEAD);
        __quicklistInsertNode(quicklist, NULL, new_node, after);
        new_node->count++;
        quicklist->count++;
        return;
    }

    /* 计算full\full_next\full_prev */
    /* 计算当前node是否已满 */
    if (!_quicklistNodeAllowInsert(node, fill, sz)) {
        D("Current node is full with count %d with requested fill %lu",
          node->count, fill);
        full = 1;
    }

    // 如果要插入的位置是给定quicklistNode的尾部之后，计算后置元素是否已满
    if (after && (entry->offset == node->count)) {
        D("At Tail of current ziplist");
        at_tail = 1;
        if (!_quicklistNodeAllowInsert(node->next, fill, sz)) {
            D("Next node is full too.");
            full_next = 1;
        }
    }
    // 如果要插入的位置是给定quicklistNode的首部之前，计算前置元素是否已满
    if (!after && (entry->offset == 0)) {
        D("At Head");
        at_head = 1;
        if (!_quicklistNodeAllowInsert(node->prev, fill, sz)) {
            D("Prev node is full too.");
            full_prev = 1;
        }
    }

    // 现在：决定插入的位置和如何进行插入
    // 1. 如果当前node未满，则解压缩当前node直接插入
    // 2. 如果当前元素已满且插入首尾且前后节点未满，则插入前后节点
    // 3. 如果当前元素已满且插入首尾单前后节点已满，则创建新node插入， 在新node插入新entry
    // 4. 如果当前元素已满且不满足以上条件，拆分当前node的ziplist为两份，在进行插入，最后尝试merger周围的5个节点
    if (!full && after) {
        D("Not full, inserting after current position.");
        quicklistDecompressNodeForUse(node);
        unsigned char *next = ziplistNext(node->zl, entry->zi);
        if (next == NULL) {
            node->zl = ziplistPush(node->zl, value, sz, ZIPLIST_TAIL);
        } else {
            node->zl = ziplistInsert(node->zl, next, value, sz);
        }
        node->count++;
        quicklistNodeUpdateSz(node);
        quicklistRecompressOnly(quicklist, node);
    } else if (!full && !after) {
        D("Not full, inserting before current position.");
        quicklistDecompressNodeForUse(node);
        node->zl = ziplistInsert(node->zl, entry->zi, value, sz);
        node->count++;
        quicklistNodeUpdateSz(node);
        quicklistRecompressOnly(quicklist, node);
    } else if (full && at_tail && node->next && !full_next && after) {
        /* If we are: at tail, next has free space, and inserting after:
         *   - insert entry at head of next node. */
        D("Full and tail, but next isn't full; inserting next node head");
        new_node = node->next;
        quicklistDecompressNodeForUse(new_node);
        new_node->zl = ziplistPush(new_node->zl, value, sz, ZIPLIST_HEAD);
        new_node->count++;
        quicklistNodeUpdateSz(new_node);
        quicklistRecompressOnly(quicklist, new_node);
    } else if (full && at_head && node->prev && !full_prev && !after) {
        /* If we are: at head, previous has free space, and inserting before:
         *   - insert entry at tail of previous node. */
        D("Full and head, but prev isn't full, inserting prev node tail");
        new_node = node->prev;
        quicklistDecompressNodeForUse(new_node);
        new_node->zl = ziplistPush(new_node->zl, value, sz, ZIPLIST_TAIL);
        new_node->count++;
        quicklistNodeUpdateSz(new_node);
        quicklistRecompressOnly(quicklist, new_node);
    } else if (full && ((at_tail && node->next && full_next && after) ||
                        (at_head && node->prev && full_prev && !after))) {
        /* If we are: full, and our prev/next is full, then:
         *   - create new node and attach to quicklist */
        D("\tprovisioning new node...");
        new_node = quicklistCreateNode();
        new_node->zl = ziplistPush(ziplistNew(), value, sz, ZIPLIST_HEAD);
        new_node->count++;
        quicklistNodeUpdateSz(new_node);
        __quicklistInsertNode(quicklist, node, new_node, after);
    } else if (full) {
        /* else, node is full we need to split it. */
        /* covers both after and !after cases */
        D("\tsplitting node...");
        quicklistDecompressNodeForUse(node);
        new_node = _quicklistSplitNode(node, entry->offset, after);
        new_node->zl = ziplistPush(new_node->zl, value, sz,
                                   after ? ZIPLIST_HEAD : ZIPLIST_TAIL);
        new_node->count++;
        quicklistNodeUpdateSz(new_node);
        __quicklistInsertNode(quicklist, node, new_node, after);
        _quicklistMergeNodes(quicklist, node);
    }

    quicklist->count++;
}

/* 拆分节点
/* 如果after==0,则拆分为[0,offset]和[offset+1,end],返回[offset+1,end],原输入参数持有[0,offset]
 * 如果after==1,则拆分为[0,offset]和[offset+1,end],返回[0,offset],原输入参数持有[offset+1,end]
 * */
REDIS_STATIC quicklistNode *_quicklistSplitNode(quicklistNode *node, int offset,
                                                int after) {
    size_t zl_sz = node->sz;

    quicklistNode *new_node = quicklistCreateNode();
    new_node->zl = zmalloc(zl_sz);

    /* 拷贝原始的ziplist */
    memcpy(new_node->zl, node->zl, zl_sz);

    /* -1 here means "continue deleting until the list ends" */
    int orig_start = after ? offset + 1 : 0;
    int orig_extent = after ? -1 : offset;
    int new_start = after ? 0 : offset;
    int new_extent = after ? offset + 1 : -1;

    D("After %d (%d); ranges: [%d, %d], [%d, %d]", after, offset, orig_start,
      orig_extent, new_start, new_extent);
    /* 删除两个ziplist中多余的元素 */

    node->zl = ziplistDeleteRange(node->zl, orig_start, orig_extent);
    node->count = ziplistLen(node->zl);
    quicklistNodeUpdateSz(node);

    new_node->zl = ziplistDeleteRange(new_node->zl, new_start, new_extent);
    new_node->count = ziplistLen(new_node->zl);
    quicklistNodeUpdateSz(new_node);

    D("After split lengths: orig (%d), new (%d)", node->count, new_node->count);
    return new_node;
}
```

### 删除：范围
```
/* 删除quicklist中的[start,start+count)元素 */
int quicklistDelRange(quicklist *quicklist, const long start,
                      const long count) {
    if (count <= 0)
        return 0;

    unsigned long extent = count; /* range is inclusive of start position */

    /* 如果删除的数量超出了quicklist，则进行截断 */
    if (start >= 0 && extent > (quicklist->count - start)) {
        /* if requesting delete more elements than exist, limit to list size. */
        extent = quicklist->count - start;
    } else if (start < 0 && extent > (unsigned long)(-start)) {
        /* else, if at negative offset, limit max size to rest of list. */
        extent = -start; /* c.f. LREM -29 29; just delete until end. */
    }

    /* 如果找不到起始节点，则直接返回 */
    quicklistEntry entry;
    if (!quicklistIndex(quicklist, start, &entry))
        return 0;

    D("Quicklist delete request for start %ld, count %ld, extent: %ld", start,
      count, extent);
    quicklistNode *node = entry.node;

    /* 遍历各个ziplist直到所有元素被删除 */
    while (extent) {
        quicklistNode *next = node->next;

        unsigned long del;
        int delete_entire_node = 0;
        if (entry.offset == 0 && extent >= node->count) {
            /* 如果当前entry偏移量为0且后续整个ziplist都要被删除，则直接删除整个ziplist所代表的quicklistNode */
            delete_entire_node = 1;
            del = node->count;
        } else if (entry.offset >= 0 && extent >= node->count) {
            /* 如果当前偏移量不为0且后续节点都需要被删除，则直接删除ziplist中的元素 */
            del = node->count - entry.offset;
        } else if (entry.offset < 0) {
            /* 如果offset<0，则表示是反向迭代，这会出现在loop的第一次 */
            /* 如果要删除的数量extent>该节点剩下的数量-entry.offset，则删除剩下的数量-entry.offset，
             * 否则删除待删除数量extent */
            del = -entry.offset;

            if (del > extent)
                del = extent;
        } else {
            /* 在其他情况下都直接删除extent个节点 */
            del = extent;
        }

        D("[%ld]: asking to del: %ld because offset: %d; (ENTIRE NODE: %d), "
          "node count: %u",
          extent, del, entry.offset, delete_entire_node, node->count);

        if (delete_entire_node) {
            __quicklistDelNode(quicklist, node);
        } else {
            // 解压缩当前节点，并删除相关数据
            quicklistDecompressNodeForUse(node);
            node->zl = ziplistDeleteRange(node->zl, entry.offset, del);
            quicklistNodeUpdateSz(node);
            node->count -= del;
            quicklist->count -= del;
            // 检查当前节点是否需要删除
            quicklistDeleteIfEmpty(quicklist, node);
            // 检查是否需要重新压缩该节点
            if (node)
                quicklistRecompressOnly(quicklist, node);
        }

        extent -= del;

        node = next;

        entry.offset = 0;
    }
    return 1;
}
```

### 修改
```
int quicklistReplaceAtIndex(quicklist *quicklist, long index, void *data,
                            int sz) {
    quicklistEntry entry;
    if (likely(quicklistIndex(quicklist, index, &entry))) {
        /* quicklistIndex provides an uncompressed node */
        /* quicklistIndex会提供一个未压缩的节点 */
        /* 先删除，再在指定位置进行插入，并更新数据结构，进行压缩 */
        entry.node->zl = ziplistDelete(entry.node->zl, &entry.zi);
        entry.node->zl = ziplistInsert(entry.node->zl, entry.zi, data, sz);
        quicklistNodeUpdateSz(entry.node);
        quicklistCompress(quicklist, entry.node);
        return 1;
    } else {
        return 0;
    }
}

/* 如果查找到，使用制定位置的元素填充entry，如果超出range，返回0表示未查找到
 * 如果entry->offset<0，表示是反向迭代
 * */
int quicklistIndex(const quicklist *quicklist, const long long idx,
                   quicklistEntry *entry) {
    quicklistNode *n;
    unsigned long long accum = 0;
    unsigned long long index;
    int forward = idx < 0 ? 0 : 1; /* < 0 -> reverse, 0+ -> forward */

    initEntry(entry);
    entry->quicklist = quicklist;

    // 根据方向进行调整
    if (!forward) {
        index = (-idx) - 1;
        n = quicklist->tail;
    } else {
        index = idx;
        n = quicklist->head;
    }

    // 如果超出range，则未找到
    if (index >= quicklist->count)
        return 0;

    // 通过每个ziplist的count直接找到对应的ziplist
    while (likely(n)) {
        if ((accum + n->count) > index) {
            break;
        } else {
            D("Skipping over (%p) %u at accum %lld", (void *)n, n->count,
              accum);
            accum += n->count;
            n = forward ? n->next : n->prev;
        }
    }

    // 如果遍历到了末尾都没有找到，返回未找到
    if (!n)
        return 0;

    D("Found node: %p at accum %llu, idx %llu, sub+ %llu, sub- %llu", (void *)n,
      accum, index, index - accum, (-index) - 1 + accum);

    entry->node = n;
    if (forward) {
        /* forward = normal head-to-tail offset. */
        entry->offset = index - accum;
    } else {
        /* reverse = need negative offset for tail-to-head, so undo
         * the result of the original if (index < 0) above. */
        entry->offset = (-index) - 1 + accum;
    }
    /* 解压缩该节点，并查找填充该节点 */
    quicklistDecompressNodeForUse(entry->node);
    entry->zi = ziplistIndex(entry->node->zl, entry->offset);
    ziplistGet(entry->zi, &entry->value, &entry->sz, &entry->longval);
    /* 调用方将会使用我们的结果，所以我们在这边不会重新压缩
     * 在需要的时候，调用方可以重新压缩或者删除该节点 */
    return 1;
}
```

### 遍历：
```
/* 返回一个offset偏移量的迭代器 */
quicklistIter *quicklistGetIteratorAtIdx(const quicklist *quicklist,
                                         const int direction,
                                         const long long idx) {
    quicklistEntry entry;

    if (quicklistIndex(quicklist, idx, &entry)) {
        quicklistIter *base = quicklistGetIterator(quicklist, direction);
        base->zi = NULL;
        base->current = entry.node;
        base->offset = entry.offset;
        return base;
    } else {
        return NULL;
    }
}

/* 注意：迭代的过程中不能插入元素，但是你可以通过quicklistDelEntry来删除当前元素。
 * 如果你在迭代过程中插入了元素，你应该重新创建迭代器
 *
 * 每次迭代都会填充entry参数。
 * */
int quicklistNext(quicklistIter *iter, quicklistEntry *entry) {
    initEntry(entry);

    if (!iter) {
        D("Returning because no iter!");
        return 0;
    }

    entry->quicklist = iter->quicklist;
    entry->node = iter->current;

    if (!iter->current) {
        D("Returning because current node is NULL")
        return 0;
    }

    unsigned char *(*nextFn)(unsigned char *, unsigned char *) = NULL;
    int offset_update = 0;

    if (!iter->zi) {
        /* 如果当前zi为空，此时表示要开始迭代一个新的ziplist，则解压当前节点，找到对应的元素在ziplist的位置 */
        quicklistDecompressNodeForUse(iter->current);
        iter->zi = ziplistIndex(iter->current->zl, iter->offset);
    } else {
        /* 如果当前zi不为空，则根据迭代方向找到前置元素或者后置元素的位置 */
        if (iter->direction == AL_START_HEAD) {
            nextFn = ziplistNext;
            offset_update = 1;
        } else if (iter->direction == AL_START_TAIL) {
            nextFn = ziplistPrev;
            offset_update = -1;
        }
        iter->zi = nextFn(iter->current->zl, iter->zi);
        iter->offset += offset_update;
    }

    entry->zi = iter->zi;
    entry->offset = iter->offset;

    if (iter->zi) {
        /* 利用该位置的数据填充entry */
        ziplistGet(entry->zi, &entry->value, &entry->sz, &entry->longval);
        return 1;
    } else {
        /* 如果新的entry为NULL，我们已经遍历完整个ziplist，选择下个node，更新offset，重新进行查找 */
        /* 压缩节点，选择下个ziplist，更新offset */
        quicklistCompress(iter->quicklist, iter->current);
        if (iter->direction == AL_START_HEAD) {
            /* Forward traversal */
            D("Jumping to start of next node");
            iter->current = iter->current->next;
            iter->offset = 0;
        } else if (iter->direction == AL_START_TAIL) {
            /* Reverse traversal */
            D("Jumping to end of previous node");
            iter->current = iter->current->prev;
            iter->offset = -1;
        }
        /* 将元素所在偏移量设置为NULL */
        iter->zi = NULL;
        return quicklistNext(iter, entry);
    }
}
```