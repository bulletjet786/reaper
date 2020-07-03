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