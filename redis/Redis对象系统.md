## 数据结构
```
/* The actual Redis Object */
#define OBJ_STRING 0    /* String object. */
#define OBJ_LIST 1      /* List object. */
#define OBJ_SET 2       /* Set object. */
#define OBJ_ZSET 3      /* Sorted set object. */
#define OBJ_HASH 4      /* Hash object. */
#define OBJ_MODULE 5    /* Module object. */
#define OBJ_STREAM 6    /* Stream object. */

#define OBJ_ENCODING_RAW 0     /* 原始sds */
#define OBJ_ENCODING_INT 1     /* 整数 */
#define OBJ_ENCODING_HT 2      /* 哈希表 */
#define OBJ_ENCODING_ZIPMAP 3  /* 压缩字典，未使用 */
#define OBJ_ENCODING_LINKEDLIST 4 /* 双端列表，不再使用了 */
#define OBJ_ENCODING_ZIPLIST 5 /* 压缩列表 */
#define OBJ_ENCODING_INTSET 6  /* 整数集合 */
#define OBJ_ENCODING_SKIPLIST 7  /* 跳表 */
#define OBJ_ENCODING_EMBSTR 8  /* 内嵌式sds */
#define OBJ_ENCODING_QUICKLIST 9 /* 快表 */
#define OBJ_ENCODING_STREAM 10 /* listpack前缀树 */

#define LRU_BITS 24
#define LRU_CLOCK_MAX ((1<<LRU_BITS)-1) /* Max value of obj->lru */
#define LRU_CLOCK_RESOLUTION 1000 /* LRU clock resolution in ms */

#define OBJ_SHARED_REFCOUNT INT_MAX
typedef struct redisObject {
    unsigned type:4;                // 对象类型
    unsigned encoding:4;            // 对象编码
    /* 当用于LRU时，表示最后一次访问时间
     * 当用于LFU时，高16位记录分钟级别访问时间，低8位记录访问频次 */
    unsigned lru:LRU_BITS;
    int refcount;
    void *ptr;
} robj;
```

## 编码转化

**String**
- 当存储的值可以表示为64位整数时，使用OBJ_ENCODING_INT实现
- 当存储的值只能用字符串表示，且其字节长度小于常量OBJ_ENCODING_EMBSTR_SIZE_LIMIT==44字节时，使用OBJ_ENCODING_EMBSTR
- 否则使用OBJ_ENCODING_RAW

```
/* 尝试编码优化一个字符串对象以节省空间 */
robj *tryObjectEncoding(robj *o) {
    long value;
    sds s = o->ptr;
    size_t len;

    /* 只对字符串类型进行编码优化 */
    serverAssertWithInfo(NULL,o,o->type == OBJ_STRING);

    /* 我们只对RAW和EMBSTR编码的对象进行优化，换句话就是使用char[]编码的对象 */
    if (!sdsEncodedObject(o)) return o;

    /* 如果该对象是一个共享对象，我们也不进行优化，对象在其他地方可能持有该对象引用 */
     if (o->refcount > 1) return o;

    /* 检查是否可以表示成一个整数，注意：我们确定大于20个字符的数字字符串表示不能转化成32或64位int */
    len = sdslen(s);
    if (len <= 20 && string2l(s,len,&value)) {
        /* 如果当前对象可以被编码为long，则尝试使用共享对象。
         * 注意：当存在最大内存开启时，我们不使用共享对象，因为每个对象都有一个
         * lru/lfu字段用于内存淘汰策略，使用了共享对象，将会使内存淘汰算法出错 */
        if ((server.maxmemory == 0 ||
            !(server.maxmemory_policy & MAXMEMORY_FLAG_NO_SHARED_INTEGERS)) &&
            value >= 0 &&
            value < OBJ_SHARED_INTEGERS)
        {
            decrRefCount(o);
            incrRefCount(shared.integers[value]);
            return shared.integers[value];
        } else {
            if (o->encoding == OBJ_ENCODING_RAW) sdsfree(o->ptr);
            o->encoding = OBJ_ENCODING_INT;
            o->ptr = (void*) value;
            return o;
        }
    }

    /* 如果字符串小于OBJ_ENCODING_EMBSTR_SIZE_LIMIT字节，我们将使用EMBSTR编码
     * 在这种情况下，所有分配的空间将会使用一个chunk从而节省空间，并提供缓存命中率 */
    if (len <= OBJ_ENCODING_EMBSTR_SIZE_LIMIT) {
        robj *emb;

        if (o->encoding == OBJ_ENCODING_EMBSTR) return o;
        emb = createEmbeddedStringObject(s,sdslen(s));
        decrRefCount(o);
        return emb;
    }

    /* 如果我们不能重编码，如果可用空间大于10%，我们对其进行截断 */
    if (o->encoding == OBJ_ENCODING_RAW &&
        sdsavail(s) > len/10)
    {
        o->ptr = sdsRemoveFreeSpace(o->ptr);
    }

    /* 返回原始对象 */
    return o;
}
```

**List**
- 使用OBJ_ENCODING_SKIPLIST实现，默认不压缩节点，单个ziplist最大为8K

**Set**
- 当entry数量小于512，且所有元素为整数时，使用OBJ_ENCODING_INTSET存储
- 否则使用OBJ_ENCODING_HT存储

**Zset**
- 当entry数量小于128，且所有entry的长度都小于64字节时，使用OBJ_ENCODING_INTSET存储
- 否则使用OBJ_ENCODING_HT存储

**Hash**
- 当entry数量小于512，且所有entry的长度都小于64字节时，使用OBJ_ENCODING_ZIPLIST存储
- 否则使用OBJ_ENCODING_HT存储

**Module**
- 无编码格式

**Stream**
- 总是使用OBJ_ENCODING_STREAM

## 内存淘汰

### 内存淘汰策略
```
# volatile-lru 从expire中按lru逐出
# allkeys-lru 从db中逐出lru
# volatile-lfu 从expire中按lfu逐出
# allkeys-lfu 从db中按lfu逐出
# volatile-random 从expire中随机逐出
# allkeys-random 从db中随机逐出
# volatile-ttl 从db中按ttl逐出
# noeviction 不逐出，对于使用更多内存的写命令返回error
```

### 更新lru/lfu：在查找key时更新
```
/* lookup的低级API，在查找键时会被调用 */
robj *lookupKey(redisDb *db, robj *key, int flags) {
    dictEntry *de = dictFind(db->dict,key->ptr);
    if (de) {
        robj *val = dictGetVal(de);

        /* 如果使用了LOOKUP_NOTOUCH标志，则不更新lru/lfu
         * 如果正在执行aof或者rdb也不更新，更新会导致写时复制 */
        if (server.rdb_child_pid == -1 &&
            server.aof_child_pid == -1 &&
            !(flags & LOOKUP_NOTOUCH))
        {
            if (server.maxmemory_policy & MAXMEMORY_FLAG_LFU) {
                updateLFU(val);
            } else {
                val->lru = LRU_CLOCK();
            }
        }
        return val;
    } else {
        return NULL;
    }
}
```

### LFU
```
/* 将当前分钟映射为一个可以保存在16位的周期的数 */
unsigned long LFUGetTimeInMinutes(void) {
    return (server.unixtime/60) & 65535;
}

/* 计算相对ldt已经过去了多长时间 */
unsigned long LFUTimeElapsed(unsigned long ldt) {
    unsigned long now = LFUGetTimeInMinutes();
    if (now >= ldt) return now-ldt;
    return 65535-ldt+now;
}

/* 对数增加计数器: counter此时不表示计数，而是计数量级层次
 * 当counter<=LFU_INIT_VAL，counter++更新的概率为1.0
 * 当counter>LFU_INIT_VAL，counter++更新的概率是为1.0/((counter-LFU_INIT_VAL)*lfu_log_factor+1)
 *              分母+1是为了防止分母为0，去除+1，上式~= 1.0/(counter-LFU_INIT_VAL) * 1.0/lfu_log_factor
 *              (counter-LFU_INIT_VAL) 为当前进入下一层次的基数，从LFU_INIT_VAL开始，分别为1/10,1/20,1/30...
 *              lfu_log_factor 为每层进入下一层次的难度因子
 * */
uint8_t LFULogIncr(uint8_t counter) {
    if (counter == 255) return 255;
    double r = (double)rand()/RAND_MAX;
    double baseval = counter - LFU_INIT_VAL;
    if (baseval < 0) baseval = 0;
    double p = 1.0/(baseval*server.lfu_log_factor+1);
    if (r < p) counter++;
    return counter;
}

/* 如果这个对象经历衰退周期，但是还没有进行衰退，则进行衰退
 * lfu_decay_time为一个衰退周期的时长，单位为分钟
 * 这个函数将会使得计数层次减少，减少量等于 衰退周期个数num_periods
 * */
unsigned long LFUDecrAndReturn(robj *o) {
    unsigned long ldt = o->lru >> 8;
    unsigned long counter = o->lru & 255;
    unsigned long num_periods = server.lfu_decay_time ? LFUTimeElapsed(ldt) / server.lfu_decay_time : 0;
    if (num_periods)
        counter = (num_periods > counter) ? 0 : counter - num_periods;
    return counter;
}
```

### LRU
```
/* 根据lru时钟单位(ms)返回一个当前的lru时钟 */
unsigned int getLRUClock(void) {
    return (mstime()/LRU_CLOCK_RESOLUTION) & LRU_CLOCK_MAX;
}

/* 如果当前lru的精度比服务器执行周期小，则直接使用server.lruclock，否则使用系统调用时间 */
unsigned int LRU_CLOCK(void) {
    unsigned int lruclock;
    // 如果server.lruclock的更新间隔(ms)小于lru时间单位(ms)，即服务器精度高于lru精度
    // 就直接使用本次执行周期的server.lruclock作为lru时钟
    // 否则使用系统调用获取实时的timestamp就是lru
    if (1000/server.hz <= LRU_CLOCK_RESOLUTION) {
        atomicGet(server.lruclock,lruclock);
    } else {
        lruclock = getLRUClock();
    }
    return lruclock;
}

// 计算对象的空闲时长，注意，lru相对时钟是循环的
unsigned long long estimateObjectIdleTime(robj *o) {
    unsigned long long lruclock = LRU_CLOCK();
    if (lruclock >= o->lru) {
        return (lruclock - o->lru) * LRU_CLOCK_RESOLUTION;
    } else {
        // 此时说明lru时钟进入了下一个循环，需要增加一个时钟周期的lru总数
        return (lruclock + (LRU_CLOCK_MAX - o->lru)) *
                    LRU_CLOCK_RESOLUTION;
    }
}
```

### 内存淘汰：在准备执行命令前：processCommand()
```
/* 当整个命令是准备好了的时候，将会执行该函数，命令参数及其数量存放在argv和argc字段中 */
/* 当命令合法，操作被执行且客户端仍处于连接状态时返回OK，否则返回ERR */
int processCommand(client *c) {
    ...

    /* 如果服务器开启了最大内存限制，处理内存达到最大相关的指令
     * 如果可以的话，首先我们尝试释放一些内存(如果存在一些易变的键在数据集中)，如果内存回收失败，我们将会返回一个ERR */
    if (server.maxmemory && !server.lua_timedout) {
        int out_of_memory = freeMemoryIfNeeded() == C_ERR;
        /* freeMemoryIfNeeded may flush slave output buffers. This may result
         * into a slave, that may be the active client, to be freed. */
        if (server.current_client == NULL) return C_ERR;

        /* It was impossible to free enough memory, and the command the client
         * is trying to execute is denied during OOM conditions? Error. */
        if ((c->cmd->flags & CMD_DENYOOM) && out_of_memory) {
            flagTransaction(c);
            addReply(c, shared.oomerr);
            return C_OK;
        }
    }

    ...
}

/* 这个函数将会定期被调用来检查是否需要根据maxmemory来释放内存，当达到了内存限制时，
 * 这个函数将会尝试通过释放内存来使得内存满足限制。
 * 如果当前满足内存限制或者释放之后满足了限制，将会返回OK，否则返回ERR
 * */
int freeMemoryIfNeeded(void) {
    /* 默认情况下，slave将会忽略maxmemory并且仅仅是master的精确拷贝 */
    if (server.masterhost && server.repl_slave_ignore_maxmemory) return C_OK;

    size_t mem_reported, mem_tofree, mem_freed;
    mstime_t latency, eviction_latency;
    long long delta;
    int slaves = listLength(server.slaves);

    /* When clients are paused the dataset should be static not just from the
     * POV of clients not being able to write, but also from the POV of
     * expires and evictions of keys not being performed. */
    if (clientsArePaused()) return C_OK;
    /* 如果不需要清理内存，则直接返回 */
    if (getMaxmemoryState(&mem_reported,NULL,&mem_tofree,NULL) == C_OK)
        return C_OK;

    mem_freed = 0;

    // 如果内存淘汰策略为不淘汰，则直接跳转到cant_free
    if (server.maxmemory_policy == MAXMEMORY_NO_EVICTION)
        goto cant_free;

    latencyStartMonitor(latency); // 开始计时
    while (mem_freed < mem_tofree) {    // 当我们释放的内存小于要释放的内存，需要一直淘汰
        int j, k, i, keys_freed = 0;
        static unsigned int next_db = 0;
        sds bestkey = NULL;
        int bestdbid;
        redisDb *db;
        dict *dict;
        dictEntry *de;

        /* 当策略是volatile-lru/allkeys-lru/volatile-lfu/allkeys-lfu/volatile-ttl时 */
        if (server.maxmemory_policy & (MAXMEMORY_FLAG_LRU|MAXMEMORY_FLAG_LFU) ||
            server.maxmemory_policy == MAXMEMORY_VOLATILE_TTL)
        {
            struct evictionPoolEntry *pool = EvictionPoolLRU;

            while(bestkey == NULL) {
                unsigned long total_keys = 0, keys;

                /* 当使用了lru/lfu/ttl时，我们将会从所有数据库中找到最应该淘汰的key */
                for (i = 0; i < server.dbnum; i++) {
                    db = server.db+i;
                    dict = (server.maxmemory_policy & MAXMEMORY_FLAG_ALLKEYS) ?
                            db->dict : db->expires;
                    // 从要检查的dict中选择一个key
                    if ((keys = dictSize(dict)) != 0) {
                        evictionPoolPopulate(i, dict, db->dict, pool);
                        total_keys += keys;
                    }
                }
                if (!total_keys) break; // 如果没有key能淘汰，我们直接break

                /* 我们从右边开始寻找待淘汰的key */
                for (k = EVPOOL_SIZE-1; k >= 0; k--) {
                    if (pool[k].key == NULL) continue;
                    bestdbid = pool[k].dbid;

                    // 如果是从MAXMEMORY_FLAG_ALLKEYS，从dict中查找，否则从expires中查找
                    if (server.maxmemory_policy & MAXMEMORY_FLAG_ALLKEYS) {
                        de = dictFind(server.db[pool[k].dbid].dict,
                            pool[k].key);
                    } else {
                        de = dictFind(server.db[pool[k].dbid].expires,
                            pool[k].key);
                    }

                    /* 如果key和cached不共用一个对象，就直接释放key */
                    if (pool[k].key != pool[k].cached)
                        sdsfree(pool[k].key);
                    pool[k].key = NULL;
                    pool[k].idle = 0;

                    /* 如果当前key还存在，我们就选择它
                     * 但是该key有可能已经被删除，是幽灵key，那么就再次从池中选择提取一个 */
                    if (de) {
                        bestkey = dictGetKey(de);
                        break;
                    } else {
                        /* 幽灵key，再次选取 */
                    }
                }
            }
        }

        /* volatile-random and allkeys-random policy */
        /* 当策略random时，即volatile-random/allkeys-random，会逐个循环淘汰各数据库元素 */
        else if (server.maxmemory_policy == MAXMEMORY_ALLKEYS_RANDOM ||
                 server.maxmemory_policy == MAXMEMORY_VOLATILE_RANDOM)
        {
            /* 当淘汰一个随机key时，我们尝试对于每一个DB逐个淘汰一个key，
             * 因此我们使用一个static变量next_db来循环访问所有DB */
            for (i = 0; i < server.dbnum; i++) {
                j = (++next_db) % server.dbnum;
                db = server.db+j;
                /* 当策略是allkeys-random我们检查db->dict，否则我们检查db->expires */
                dict = (server.maxmemory_policy == MAXMEMORY_ALLKEYS_RANDOM) ?
                        db->dict : db->expires;
                // 提取一个随机key来删除
                if (dictSize(dict) != 0) {
                    de = dictGetRandomKey(dict);
                    bestkey = dictGetKey(de);
                    bestdbid = j;
                    break;
                }
            }
        }

        /* 最后我们移除这个选择的key */
        if (bestkey) {
            db = server.db+bestdbid;
            robj *keyobj = createStringObject(bestkey,sdslen(bestkey));
            // 传播过期信息
            propagateExpire(db,keyobj,server.lazyfree_lazy_eviction);
            /* We compute the amount of memory freed by db*Delete() alone.
             * It is possible that actually the memory needed to propagate
             * the DEL in AOF and replication link is greater than the one
             * we are freeing removing the key, but we can't account for
             * that otherwise we would never exit the loop.
             *
             * AOF and Output buffer memory will be freed eventually so
             * we only care about memory used by the key space. */
            delta = (long long) zmalloc_used_memory();
            latencyStartMonitor(eviction_latency);
            if (server.lazyfree_lazy_eviction)
                dbAsyncDelete(db,keyobj);
            else
                dbSyncDelete(db,keyobj);
            latencyEndMonitor(eviction_latency);
            latencyAddSampleIfNeeded("eviction-del",eviction_latency);
            latencyRemoveNestedEvent(latency,eviction_latency);
            delta -= (long long) zmalloc_used_memory();
            mem_freed += delta;
            server.stat_evictedkeys++;
            // 发送键空间通知
            notifyKeyspaceEvent(NOTIFY_EVICTED, "evicted",
                keyobj, db->id);
            decrRefCount(keyobj);
            keys_freed++;

            /* 如果要释放的内存很多，我们就无法在很短时间传输数据，会导致主从同步延迟很大，
             * 所以我们强制在循环内部刷新主从同步 */
            if (slaves) flushSlavesOutputBuffers();

            /* Normally our stop condition is the ability to release
             * a fixed, pre-computed amount of memory. However when we
             * are deleting objects in another thread, it's better to
             * check, from time to time, if we already reached our target
             * memory, since the "mem_freed" amount is computed only
             * across the dbAsyncDelete() call, while the thread can
             * release the memory all the time. */
            if (server.lazyfree_lazy_eviction && !(keys_freed % 16)) {
                if (getMaxmemoryState(NULL,NULL,NULL,NULL) == C_OK) {
                    /* Let's satisfy our stop condition. */
                    mem_freed = mem_tofree;
                }
            }
        }

        // 如果在某次循环中，没有释放任何key，则说明我们无法释放更多内存了，转到cant_free
        if (!keys_freed) {
            latencyEndMonitor(latency);
            latencyAddSampleIfNeeded("eviction-cycle",latency);
            goto cant_free; /* nothing to free... */
        }
    }
    latencyEndMonitor(latency);
    latencyAddSampleIfNeeded("eviction-cycle",latency);
    return C_OK;

cant_free:
    /* 我们没有办法释放内存了，我们唯一能做的就是不断检查后台惰性key释放任务
     * 当后台惰性key释放任务没有更多任务时，仍没有满足内存限制，就返回ERR */
    while(bioPendingJobsOfType(BIO_LAZY_FREE)) {
        if (((mem_reported - zmalloc_used_memory()) + mem_freed) >= mem_tofree)
            break;
        usleep(1000);
    }
    return C_ERR;
}

/* 使用抽样数据填充淘汰池 */
void evictionPoolPopulate(int dbid, dict *sampledict, dict *keydict, struct evictionPoolEntry *pool) {
    int j, k, count;
    dictEntry *samples[server.maxmemory_samples];

    // 从dict中取样最多server.maxmemory_samples个数据放进samples数组中，实际返回数量为count
    count = dictGetSomeKeys(sampledict,samples,server.maxmemory_samples);
    for (j = 0; j < count; j++) {
        unsigned long long idle;
        sds key;
        robj *o;
        dictEntry *de;

        de = samples[j];
        key = dictGetKey(de);

        /* 当我们需要不是通过ttl机制来计算是，且取样集合不是数据集合时，我们需要取到原数据 */
        if (server.maxmemory_policy != MAXMEMORY_VOLATILE_TTL) {
            if (sampledict != keydict) de = dictFind(keydict, key);
            o = dictGetVal(de);
        }

        /* 计算对应策略的idle
         * 这个数字之所以叫做idle仅仅是因为这个代码最初用于处理lru，但是实际上现在它只是一个分值，
         * 分值越大的越早淘汰 */
        if (server.maxmemory_policy & MAXMEMORY_FLAG_LRU) {
            /* 当使用lru时，我们使用空转时长 */
            idle = estimateObjectIdleTime(o);
        } else if (server.maxmemory_policy & MAXMEMORY_FLAG_LFU) {
            /* 当使用lfu时，counter为当前的计数层次，最大为255，counter大的越频繁访问，越不应该被淘汰，使用反数 */
            idle = 255-LFUDecrAndReturn(o);
        } else if (server.maxmemory_policy == MAXMEMORY_VOLATILE_TTL) {
            /* 当使用ttl时，过期时间越大的越不应该被淘汰，使用反数 */
            idle = ULLONG_MAX - (long)dictGetVal(de);
        } else {
            serverPanic("Unknown eviction policy in evictionPoolPopulate()");
        }

        /* 插入到淘汰池中 */
        k = 0;
        while (k < EVPOOL_SIZE &&
               pool[k].key &&
               pool[k].idle < idle) k++;
        // k为要插入的位置
        // 1. 回收池满, 且不能插入:  k == 0 && pool[EVPOOL_SIZE-1].key != NULL：抛弃
        // 2. 回收池未满，且可以插入，且拆入位置元素为NULL：无需调整
        // 3. 回收池未满，且可以插入，且拆入位置元素不为NULL：右移插入
        // 4. 回收池满，但可以拆入：左移插入
        if (k == 0 && pool[EVPOOL_SIZE-1].key != NULL) {
            continue;
        } else if (k < EVPOOL_SIZE && pool[k].key == NULL) {
        } else {
            if (pool[EVPOOL_SIZE-1].key == NULL) {
                sds cached = pool[EVPOOL_SIZE-1].cached;
                memmove(pool+k+1,pool+k,
                    sizeof(pool[0])*(EVPOOL_SIZE-k-1));
                pool[k].cached = cached;
            } else {
                k--;
                if (pool[0].key != pool[0].cached) sdsfree(pool[0].key);
                memmove(pool,pool+1,sizeof(pool[0])*k);
                pool[k].cached = cached;
            }
        }

        /* 考虑到sds对象的创建的成本，当新的key的大小不大于EVPOOL_CACHED_SDS_SIZE，
         * 则直接重用cached, 所有的cached字段在evictionPoolAlloc被初始化 */
        int klen = sdslen(key);
        if (klen > EVPOOL_CACHED_SDS_SIZE) {
            pool[k].key = sdsdup(key);
        } else {
            memcpy(pool[k].cached,key,klen+1);
            sdssetlen(pool[k].cached,klen);
            pool[k].key = pool[k].cached;
        }
        pool[k].idle = idle;
        pool[k].dbid = dbid;
    }
}

/* 从maxmemory指令的角度获取内存状态，如果内存满足限制返回OK，否则返回ERR
 * 这个函数通过非NULL的入参引用将会返回其他的信息
 *
 * total: 总共的内存使用量，ERR和OK时填充
 * logical：总共的内存使用量 减去 主从复制output buffer和AOF buffer，ERR时填充
 * tofree：需要释放的内存
 * level：当前内存使用量/maxmemory
 * */
int getMaxmemoryState(size_t *total, size_t *logical, size_t *tofree, float *level) {
    size_t mem_reported, mem_used, mem_tofree;

    /* Check if we are over the memory usage limit. If we are not, no need
     * to subtract the slaves output buffers. We can just return ASAP. */
    mem_reported = zmalloc_used_memory();
    if (total) *total = mem_reported;

    /* We may return ASAP if there is no need to compute the level. */
    int return_ok_asap = !server.maxmemory || mem_reported <= server.maxmemory;
    if (return_ok_asap && !level) return C_OK;

    /* logic内存不包括 slaves output buffer 和 AOF buffer */
    mem_used = mem_reported;
    size_t overhead = freeMemoryGetNotCountedMemory();
    mem_used = (mem_used > overhead) ? mem_used-overhead : 0;

    /* Compute the ratio of memory usage. */
    if (level) {
        if (!server.maxmemory) {
            *level = 0;
        } else {
            *level = (float)mem_used / (float)server.maxmemory;
        }
    }

    if (return_ok_asap) return C_OK;

    /* Check if we are still over the memory limit. */
    if (mem_used <= server.maxmemory) return C_OK;

    /* Compute how much memory we need to free. */
    mem_tofree = mem_used - server.maxmemory;

    if (logical) *logical = mem_used;
    if (tofree) *tofree = mem_tofree;

    return C_ERR;
}

/* 为了提高LRU近似算法的质量，我们使用为freeMemoryIfNeeded函数提供一个候选集合
 * 在淘汰池中的entry将会按照idle time排序，大idle time的在右边
 * */
#define EVPOOL_SIZE 16
#define EVPOOL_CACHED_SDS_SIZE 255
struct evictionPoolEntry {
    unsigned long long idle;    /* Object idle time (inverse frequency for LFU) */
    sds key;                    /* Key name. */
    sds cached;                 /* Cached SDS object for key name. */
    int dbid;                   /* Key DB number. */
};

// 全局淘汰池
static struct evictionPoolEntry *EvictionPoolLRU;
```

## 对象共享

### 引用计数
- 当引用计数为OBJ_SHARED_REFCOUNT时，表示共享对象
- 在对象系统之外，将会使用incrRefCount和decrRefCount函数来进行对象的创建和删除，而不是直接调用freeStringObject或者freeListObject
- 利用引用计数可以实现一种简单的内存回收机制，redis可以通过跟踪对象的引用计数信息，在适当的时候自动释放对象并进行内存回收。
- 利用引用计数，还可以实现对象间的共享，redis把[0,10000)间的字符串对象使用池化进行了全局共享。
```
#define OBJ_SHARED_REFCOUNT INT_MAX

void incrRefCount(robj *o) {
    if (o->refcount != OBJ_SHARED_REFCOUNT) o->refcount++;
}

void decrRefCount(robj *o) {
    if (o->refcount == 1) {
        switch(o->type) {
        case OBJ_STRING: freeStringObject(o); break;
        case OBJ_LIST: freeListObject(o); break;
        case OBJ_SET: freeSetObject(o); break;
        case OBJ_ZSET: freeZsetObject(o); break;
        case OBJ_HASH: freeHashObject(o); break;
        case OBJ_MODULE: freeModuleObject(o); break;
        case OBJ_STREAM: freeStreamObject(o); break;
        default: serverPanic("Unknown object type"); break;
        }
        zfree(o);
    } else {
        if (o->refcount <= 0) serverPanic("decrRefCount against refcount <= 0");
        if (o->refcount != OBJ_SHARED_REFCOUNT) o->refcount--;
    }
}

robj *makeObjectShared(robj *o) {
    serverAssert(o->refcount == 1);
    o->refcount = OBJ_SHARED_REFCOUNT;
    return o;
}
```

### 全局对象
sharedObjectsStruct其实并不表示共享对象，而是全局对象
在sharedObjectsStruct中，其他对象仅仅只是直接创建的，这些对象不会被删除，但是integers却是通过makeObejctShared全局共享的
```
// redis共享对象，相当于全局变量
struct sharedObjectsStruct {
    robj *crlf, *ok, *err, *emptybulk, *czero, *cone, *cnegone, *pong, *space,
    *colon, *nullbulk, *nullmultibulk, *queued,
    *emptymultibulk, *wrongtypeerr, *nokeyerr, *syntaxerr, *sameobjecterr,
    *outofrangeerr, *noscripterr, *loadingerr, *slowscripterr, *bgsaveerr,
    *masterdownerr, *roslaveerr, *execaborterr, *noautherr, *noreplicaserr,
    *busykeyerr, *oomerr, *plus, *messagebulk, *pmessagebulk, *subscribebulk,
    *unsubscribebulk, *psubscribebulk, *punsubscribebulk, *del, *unlink,
    *rpop, *lpop, *lpush, *rpoplpush, *zpopmin, *zpopmax, *emptyscan,
    *select[PROTO_SHARED_SELECT_CMDS],
    *integers[OBJ_SHARED_INTEGERS],
    *mbulkhdr[OBJ_SHARED_BULKHDR_LEN], /* "*<value>\r\n" */
    *bulkhdr[OBJ_SHARED_BULKHDR_LEN];  /* "$<value>\r\n" */
    sds minstring, maxstring;
};
struct sharedObjectsStruct shared;

/* 初始化shared对象 */
void createSharedObjects(void) {
    int j;

    /* 服务端回复相关: redisObject<<OBJ_STRING>> */
    shared.crlf = createObject(OBJ_STRING,sdsnew("\r\n"));
    shared.ok = createObject(OBJ_STRING,sdsnew("+OK\r\n"));
    shared.err = createObject(OBJ_STRING,sdsnew("-ERR\r\n"));
    shared.emptybulk = createObject(OBJ_STRING,sdsnew("$0\r\n\r\n"));
    shared.czero = createObject(OBJ_STRING,sdsnew(":0\r\n"));
    shared.cone = createObject(OBJ_STRING,sdsnew(":1\r\n"));
    shared.cnegone = createObject(OBJ_STRING,sdsnew(":-1\r\n"));
    shared.nullbulk = createObject(OBJ_STRING,sdsnew("$-1\r\n"));
    shared.nullmultibulk = createObject(OBJ_STRING,sdsnew("*-1\r\n"));
    shared.emptymultibulk = createObject(OBJ_STRING,sdsnew("*0\r\n"));
    shared.pong = createObject(OBJ_STRING,sdsnew("+PONG\r\n"));
    shared.queued = createObject(OBJ_STRING,sdsnew("+QUEUED\r\n"));
    shared.emptyscan = createObject(OBJ_STRING,sdsnew("*2\r\n$1\r\n0\r\n*0\r\n"));
    shared.wrongtypeerr = createObject(OBJ_STRING,sdsnew(
        "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"));
    shared.nokeyerr = createObject(OBJ_STRING,sdsnew(
        "-ERR no such key\r\n"));
    shared.syntaxerr = createObject(OBJ_STRING,sdsnew(
        "-ERR syntax error\r\n"));
    shared.sameobjecterr = createObject(OBJ_STRING,sdsnew(
        "-ERR source and destination objects are the same\r\n"));
    shared.outofrangeerr = createObject(OBJ_STRING,sdsnew(
        "-ERR index out of range\r\n"));
    shared.noscripterr = createObject(OBJ_STRING,sdsnew(
        "-NOSCRIPT No matching script. Please use EVAL.\r\n"));
    shared.loadingerr = createObject(OBJ_STRING,sdsnew(
        "-LOADING Redis is loading the dataset in memory\r\n"));
    shared.slowscripterr = createObject(OBJ_STRING,sdsnew(
        "-BUSY Redis is busy running a script. You can only call SCRIPT KILL or SHUTDOWN NOSAVE.\r\n"));
    shared.masterdownerr = createObject(OBJ_STRING,sdsnew(
        "-MASTERDOWN Link with MASTER is down and replica-serve-stale-data is set to 'no'.\r\n"));
    shared.bgsaveerr = createObject(OBJ_STRING,sdsnew(
        "-MISCONF Redis is configured to save RDB snapshots, but it is currently not able to persist on disk. Commands that may modify the data set are disabled, because this instance is configured to report errors during writes if RDB snapshotting fails (stop-writes-on-bgsave-error option). Please check the Redis logs for details about the RDB error.\r\n"));
    shared.roslaveerr = createObject(OBJ_STRING,sdsnew(
        "-READONLY You can't write against a read only replica.\r\n"));
    shared.noautherr = createObject(OBJ_STRING,sdsnew(
        "-NOAUTH Authentication required.\r\n"));
    shared.oomerr = createObject(OBJ_STRING,sdsnew(
        "-OOM command not allowed when used memory > 'maxmemory'.\r\n"));
    shared.execaborterr = createObject(OBJ_STRING,sdsnew(
        "-EXECABORT Transaction discarded because of previous errors.\r\n"));
    shared.noreplicaserr = createObject(OBJ_STRING,sdsnew(
        "-NOREPLICAS Not enough good replicas to write.\r\n"));
    shared.busykeyerr = createObject(OBJ_STRING,sdsnew(
        "-BUSYKEY Target key name already exists.\r\n"));
    shared.space = createObject(OBJ_STRING,sdsnew(" "));
    shared.colon = createObject(OBJ_STRING,sdsnew(":"));
    shared.plus = createObject(OBJ_STRING,sdsnew("+"));

    for (j = 0; j < PROTO_SHARED_SELECT_CMDS; j++) {
        char dictid_str[64];
        int dictid_len;

        dictid_len = ll2string(dictid_str,sizeof(dictid_str),j);
        shared.select[j] = createObject(OBJ_STRING,
            sdscatprintf(sdsempty(),
                "*2\r\n$6\r\nSELECT\r\n$%d\r\n%s\r\n",
                dictid_len, dictid_str));
    }
    shared.messagebulk = createStringObject("$7\r\nmessage\r\n",13);
    shared.pmessagebulk = createStringObject("$8\r\npmessage\r\n",14);
    shared.subscribebulk = createStringObject("$9\r\nsubscribe\r\n",15);
    shared.unsubscribebulk = createStringObject("$11\r\nunsubscribe\r\n",18);
    shared.psubscribebulk = createStringObject("$10\r\npsubscribe\r\n",17);
    shared.punsubscribebulk = createStringObject("$12\r\npunsubscribe\r\n",19);
    shared.del = createStringObject("DEL",3);
    shared.unlink = createStringObject("UNLINK",6);
    shared.rpop = createStringObject("RPOP",4);
    shared.lpop = createStringObject("LPOP",4);
    shared.lpush = createStringObject("LPUSH",5);
    shared.rpoplpush = createStringObject("RPOPLPUSH",9);
    shared.zpopmin = createStringObject("ZPOPMIN",7);
    shared.zpopmax = createStringObject("ZPOPMAX",7);
    for (j = 0; j < OBJ_SHARED_INTEGERS; j++) {
        shared.integers[j] =
            makeObjectShared(createObject(OBJ_STRING,(void*)(long)j));
        shared.integers[j]->encoding = OBJ_ENCODING_INT;
    }
    for (j = 0; j < OBJ_SHARED_BULKHDR_LEN; j++) {
        shared.mbulkhdr[j] = createObject(OBJ_STRING,
            sdscatprintf(sdsempty(),"*%d\r\n",j));
        shared.bulkhdr[j] = createObject(OBJ_STRING,
            sdscatprintf(sdsempty(),"$%d\r\n",j));
    }
    /* The following two shared objects, minstring and maxstrings, are not
     * actually used for their value but as a special object meaning
     * respectively the minimum possible string and the maximum possible
     * string in string comparisons for the ZRANGEBYLEX command. */
    shared.minstring = sdsnew("minstring");
    shared.maxstring = sdsnew("maxstring");
}
```
