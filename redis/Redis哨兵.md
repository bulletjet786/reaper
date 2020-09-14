## 哨兵机制

### 基本原理
#### 命令发送
- sentinel每10s每个Sentinel向master\slaves发送INFO命令
    - 发现salve节点
    - 确认主从关系
- sentinel每2s每个Sentinel向master\slaves\sentinels用PUBLISH发布信息
    - 发布对节点的“看法”和自身信息
    - 发现其他sentinel节点
- sentinel每1s每个sentinel向master\slaves\sentinels发送PING命令
    - 心跳检测，下线判定的依据
- 客户端将sentinel作为redis发现的配置中心，也通过sentinel的频道作为配置变更的提醒机制
    - SENTINEL GET-MASTER-ADDR-BY-NAME获取主节点信息
    - SENTINEL SLAVES获取从节点信息
    - 通过switch-master频道获取主备切换的信息
    - 通过convert-to-slave, +sdown, +slave, +slave-reconf-done等频道获取slave节点配置变更信息

#### 命令接收
- 收到master\slaves的INFO回复
- 收到master\slaves\sentinels对PING命令的回复
- 收到订阅频道master/slaves的__sentinel__:hello的订阅信息，和其它sentinels用PUBLISH发来的信息

### 流程图
![哨兵流程图](https://tva1.sinaimg.cn/large/007S8ZIlly1giq1bgazgaj30ur0u0dis.jpg)

### 源码分析
#### 相关常量和结构体
```
/* 正在监控的RedisInstance对象 */
#define SRI_MASTER  (1<<0)  /* master */
#define SRI_SLAVE   (1<<1)  /* slave */
#define SRI_SENTINEL (1<<2) /* sentinel */
#define SRI_S_DOWN (1<<3)   /* 主观下线 */
#define SRI_O_DOWN (1<<4)   /* 客观下线 */
#define SRI_MASTER_DOWN (1<<5) /* 当前sentinel认为master下线 */
#define SRI_FAILOVER_IN_PROGRESS (1<<6) /* 对于这个master，正在进行故障转移 */
#define SRI_PROMOTED (1<<7)            /* 这个slave已经被选为待晋升 */
#define SRI_RECONF_SENT (1<<8)     /* SLAVEOF <newmaster>已经发送. */
#define SRI_RECONF_INPROG (1<<9)   /* slave正在同步master */
#define SRI_RECONF_DONE (1<<10)     /* slave已经完成到master的同步 */
#define SRI_FORCE_FAILOVER (1<<11)  /* Force failover with master up. */
#define SRI_SCRIPT_KILL_SENT (1<<12) /* SCRIPT KILL already sent on -BUSY */

/* Note: times are in milliseconds. */
#define SENTINEL_INFO_PERIOD 10000
#define SENTINEL_PING_PERIOD 1000
#define SENTINEL_ASK_PERIOD 1000
#define SENTINEL_PUBLISH_PERIOD 2000
#define SENTINEL_DEFAULT_DOWN_AFTER 30000
#define SENTINEL_HELLO_CHANNEL "__sentinel__:hello"
#define SENTINEL_TILT_TRIGGER 2000
#define SENTINEL_TILT_PERIOD (SENTINEL_PING_PERIOD*30)
#define SENTINEL_DEFAULT_SLAVE_PRIORITY 100
#define SENTINEL_SLAVE_RECONF_TIMEOUT 10000
#define SENTINEL_DEFAULT_PARALLEL_SYNCS 1
#define SENTINEL_MIN_LINK_RECONNECT_PERIOD 15000
#define SENTINEL_DEFAULT_FAILOVER_TIMEOUT (60*3*1000)
#define SENTINEL_MAX_PENDING_COMMANDS 100
#define SENTINEL_ELECTION_TIMEOUT 10000
#define SENTINEL_MAX_DESYNC 1000
#define SENTINEL_DEFAULT_DENY_SCRIPTS_RECONFIG 1

/* Failover machine different states. */
#define SENTINEL_FAILOVER_STATE_NONE 0  /* 没有故障转移在进行中 */
#define SENTINEL_FAILOVER_STATE_WAIT_START 1  /* 等待开始一个故障转移 Wait for failover_start_time*/
#define SENTINEL_FAILOVER_STATE_SELECT_SLAVE 2 /* 正选择一个slave晋升 */
#define SENTINEL_FAILOVER_STATE_SEND_SLAVEOF_NOONE 3 /* 发送slaveof no one给slave */
#define SENTINEL_FAILOVER_STATE_WAIT_PROMOTION 4 /* 等待所选择的slave晋升完成 */
#define SENTINEL_FAILOVER_STATE_RECONF_SLAVES 5 /* 正在重新配置其它salves：SLAVEOF newmaster */
#define SENTINEL_FAILOVER_STATE_UPDATE_CONFIG 6 /* 其它slave重新配置完成，监视晋升的slave . */

#define SENTINEL_MASTER_LINK_STATUS_UP 0
#define SENTINEL_MASTER_LINK_STATUS_DOWN 1

/* 一条到sentinelRedisInstance的连接。我们可能会有一个sentinel集合监控很多master。
 * 比如5个sentinel监控100个master，那么对于100个master如果我们都各创建5个到sentinel的instanceLink，
 * 我们将会创建500个instanceLink，但实际上我们只需要创建5个到sentinel的instanceLink共享就行了，
 * refcount被用来实现共享，这样不仅用5个instanceLink代替了500个instanceLink，还用5个PING代替了500个PING。
 *
 * instanceLink的共享仅用于sentinels，master和slave的refcount总是1。 */
typedef struct instanceLink {
    int refcount;          /* 这个连接被多少sentinelRedisInstance共用 */
    int disconnected;      /* 如果cc或pc连接需要重连 */
    int pending_commands;  /* 这条连接上正在等待reply的数量 */
    redisAsyncContext *cc; /* 命令连接的Hiredis上下文 */
    redisAsyncContext *pc; /* 命令连接的Hiredis上下文 */
    mstime_t cc_conn_time; /* cc连接的时间 */
    mstime_t pc_conn_time; /* pc连接的时间 */
    mstime_t pc_last_activity; /* 我们最后收到消息的时间 */
    mstime_t last_avail_time; /* 最后一次这个实例收到一个合法的PING回复的时间 */
    /* 正在等待回复的最后一次PING的时间。
     * 当收到回复时时设置为0
     * 当值为0并发送一个新的PING时设置为当前时间， */
    mstime_t act_ping_time;
    /* 最后一次发送ping的时间，仅用于防止在失败时发送过多的PING。空闲时间使用act_ping_time计算。 */
    mstime_t last_ping_time;
    /* 收到上一次回复的时间，无论收到的回复是什么。用来检查连接是否空闲，从而必须重连。 */
    mstime_t last_pong_time;
    mstime_t last_reconn_time;  /* 当连接段开始，最后一次企图重连的时间 */
} instanceLink;

typedef struct sentinelRedisInstance {
    int flags;      /* SRI_...标志 */
    char *name;     /* 从这个sentinel视角看到的master名 */
    char *runid;    /* 这个实例的runid或者这个sentinel的uniqueID */
    uint64_t config_epoch;  /* 配置纪元 */
    sentinelAddr *addr; /* Master host. */
    instanceLink *link; /* 到这个实例的连接，对于sentinels可能是共享的 */
    mstime_t last_pub_time;   /* 最后一次我们我们发送hello的时间 */
    mstime_t last_hello_time; /* 仅Sentinel使用，最后一次我们收到hello消息的时间 */
    mstime_t last_master_down_reply_time; /* 最后一次收到SENTINEL is-master-down的回复时间 */
    mstime_t s_down_since_time; /* Subjectively down since time. */
    mstime_t o_down_since_time; /* Objectively down since time. */
    mstime_t down_after_period; /* 如果经历了 Consider it down after that period. */
    mstime_t info_refresh;  /* 我们最后一次收到INFO回复的时间点 */
    dict *renamed_commands;     /* 重命名的命令 */
    
    /* 角色和我们第一次观察到变更为该角色的时间。
     * 在延迟替换时这是很有用的。我们需要等待一段时间来给新的leader报告新的配置。 */
    int role_reported;
    mstime_t role_reported_time;
    mstime_t slave_conf_change_time; /* 最后一次slave的master地址改变的时间点 */

    /* master相关 */
    dict *sentinels;    /* 监视相同的master的其他sentinels */
    dict *slaves;       /* 这个master的slaves */
    unsigned int quorum;/* 故障转移时需要统一的sentinels数量 */
    int parallel_syncs; /* 同时有多少slaves进行重配置。 */
    char *auth_pass;    /* 验证master和slaves时需要提供的密码 */

    /* slave相关 */
    mstime_t master_link_down_time; /* slave的复制连接下线的时间 */
    int slave_priority; /* 根据INFO输出的slave的优先级 */
    mstime_t slave_reconf_sent_time; /* 发送SLAVE OF <new>的时间 */
    struct sentinelRedisInstance *master; /* 如果是slave，这个字段表示她的master */
    char *slave_master_host;    /* INFO报告的master host */
    int slave_master_port;      /* INFO报告的master port */
    int slave_master_link_status; /* Master link status as reported by INFO */
    unsigned long long slave_repl_offset; /* Slave replication offset. */
    /* Failover */
    char *leader;       /* 如果是master，表示执行故障转移时的runid，如果是sentinel，表示这个sentinel投票的leader */
    uint64_t leader_epoch; /* leader的配置纪元 */
    uint64_t failover_epoch; /* 当前执行的故障转移的配置纪元 */
    int failover_state; /* See SENTINEL_FAILOVER_STATE_* defines. */
    mstime_t failover_state_change_time;
    mstime_t failover_start_time;   /* 最后一次故障转移企图开始的时间 */
    mstime_t failover_timeout;      /* 刷新故障转移状态的最大时间 */
    mstime_t failover_delay_logged; /* For what failover_start_time value we
                                       logged the failover delay. */
    struct sentinelRedisInstance *promoted_slave; /* 晋升的slave实例 */
    /* 用来提醒管理员和重新配置客户端的脚本：如果为NULL则没有脚本会执行 */
    char *notification_script;
    char *client_reconfig_script;
    sds info; /* cached INFO output */
} sentinelRedisInstance;

/* Sentinel状态 */
struct sentinelState {
    char myid[CONFIG_RUN_ID_SIZE + 1]; /* 这个Sentinel的ID */
    uint64_t current_epoch;         /* 当前纪元 */
    dict *masters;      /* 所有master的字典。key是实例名，value是sentinelRedisInstance指针 */
    int tilt;           /* 我们是否处于TILT模式? */
    int running_scripts;    /* 正在执行的脚本的数量 */
    mstime_t tilt_start_time;       /* TILT模式开始时间 */
    mstime_t previous_time;         /* 上次运行定时器函数的时间 */
    list *scripts_queue;            /* 等待执行的用户脚本队列 */
    char *announce_ip;  /* gossiped给其它sentinels的IP地址 */
    int announce_port;  /* gossiped给其它sentinels的IP端口 */
    unsigned long simfailure_flags; /* 模拟故障转移 */
    int deny_scripts_reconfig; /* 是否运行通过Allow SENTINEL SET ...在运行时改变脚本地址 */
} sentinel;
```

#### 初始化
```
int main(int argc, char **argv) {
    ...

    /* 初始化端口号常量和数据结构 */
    if (server.sentinel_mode) {
        initSentinelConfig();
        initSentinel();
    }

    ...

    if (!server.sentinel_mode) {
        ...
    } else {
        sentinelIsRunning();
    }
    
    ...

    aeSetBeforeSleepProc(server.el,beforeSleep);
    aeSetAfterSleepProc(server.el,afterSleep);
    aeMain(server.el);
    aeDeleteEventLoop(server.el);
    return 0;
}

/* 这个函数会在server运行在Sentinel模式下时调用，来启动、加载配置和为正常操作做准备 */
void sentinelIsRunning(void) {
    int j;

    if (server.configfile == NULL) {
        serverLog(LL_WARNING,
                  "Sentinel started without a config file. Exiting...");
        exit(1);
    } else if (access(server.configfile, W_OK) == -1) {
        serverLog(LL_WARNING,
                  "Sentinel config file %s is not writable: %s. Exiting...",
                  server.configfile, strerror(errno));
        exit(1);
    }

    /* 如果Sentinel在配置文件中还没有一个ID，我们将会随机生成一个并持久化到磁盘上。
     * 从现在开始，即使重启也会使用同一个ID */
    for (j = 0; j < CONFIG_RUN_ID_SIZE; j++)
        if (sentinel.myid[j] != 0) break;

    if (j == CONFIG_RUN_ID_SIZE) {
        /* 选择一个ID并持久化到配置文件中 */
        getRandomHexChars(sentinel.myid, CONFIG_RUN_ID_SIZE);
        sentinelFlushConfig();
    }

    /* Log its ID to make debugging of issues simpler. */
    serverLog(LL_WARNING, "Sentinel ID is %s", sentinel.myid);

    /* 在启动时，对于每个配置的master，我们想要生成一个+monitor的事件 */
    sentinelGenerateInitialMonitorEvents();
}

void sentinelTimer(void) {
    sentinelCheckTiltCondition();   // 检查TILT条件
    sentinelHandleDictOfRedisInstances(sentinel.masters);
    sentinelRunPendingScripts();
    sentinelCollectTerminatedScripts();
    sentinelKillTimedoutScripts();

    /* 我们持续的修改Redis定时器中断的频率是为了防止各个sentinel的定时器同步，
     * 从而降低同时发起领导选举的可能性 */
    server.hz = CONFIG_DEFAULT_HZ + rand() % CONFIG_DEFAULT_HZ;
}

/* 这个函数检查我们是否需要进入TITL模式。
 * 当我们在两次时间中断调用中遇到以下情况时，如果调用时间差为负或者超过了2s，我们会进入TITL模式。
 * 注意：如果我们认为100ms左右是正常的，如果我们需要进入TITL模式，说明我们遇到了以下情况：
 * 1) Sentinel进程因为某些原因阻塞住了，可能是：负载太高，IO冻结，信号停止等等。
 * 2) 系统时钟被修改了。
 * 在上面两种情况下Sentinel会认为出现了超时甚至故障，这是我们进入TILT，并且在SENTINEL_TILT_PERIOD
 * 时间内我们都不执行任何操作。
 * 在TILT期间，我们仍然收集信息，但是我们不执行操作。 */
void sentinelCheckTiltCondition(void) {
    mstime_t now = mstime();
    mstime_t delta = now - sentinel.previous_time;

    if (delta < 0 || delta > SENTINEL_TILT_TRIGGER) {
        sentinel.tilt = 1;
        sentinel.tilt_start_time = mstime();
        sentinelEvent(LL_WARNING, "+tilt", NULL, "#tilt mode entered");
    }
    sentinel.previous_time = mstime();
}

/* 对所有监控的master执行调度操作。 */
void sentinelHandleDictOfRedisInstances(dict *instances) {
    dictIterator *di;
    dictEntry *de;
    sentinelRedisInstance *switch_to_promoted = NULL;

    /* 对于每个master，有一些额外的事情要执行 */
    di = dictGetIterator(instances);
    while ((de = dictNext(di)) != NULL) {
        sentinelRedisInstance *ri = dictGetVal(de);

        sentinelHandleRedisInstance(ri);
        if (ri->flags & SRI_MASTER) {
            // 如果是master，需要递归的处理slaves和sentinels
            sentinelHandleDictOfRedisInstances(ri->slaves);
            sentinelHandleDictOfRedisInstances(ri->sentinels);
            if (ri->failover_state == SENTINEL_FAILOVER_STATE_UPDATE_CONFIG) {
                switch_to_promoted = ri;
            }
        }
    }
    if (switch_to_promoted)
        sentinelFailoverSwitchToPromotedSlave(switch_to_promoted);
    dictReleaseIterator(di);
}

/* 对指定的Redis实例执行调度操作 */
void sentinelHandleRedisInstance(sentinelRedisInstance *ri) {
    /* ========== MONITORING HALF ============ */
    /* 对每种实例都要执行 */
    sentinelReconnectInstance(ri);          // 如果断开，重连
    sentinelSendPeriodicCommands(ri);       // 发送周期命令

    /* ============== ACTING HALF ============= */
    /* 如果我们在TILT模式，则不会执行故障转移的相关操作 */
    if (sentinel.tilt) {
        if (mstime() - sentinel.tilt_start_time < SENTINEL_TILT_PERIOD) return;
        sentinel.tilt = 0;
        sentinelEvent(LL_WARNING, "-tilt", NULL, "#tilt mode exited");
    }

    /* 对master、slave或sentinel检查是否客观下线 */
    sentinelCheckSubjectivelyDown(ri);

    /* 如果是master或者slave */
    if (ri->flags & (SRI_MASTER | SRI_SLAVE)) {
        /* Nothing so far. */
    }

    /* 如果当前实例是master */
    if (ri->flags & SRI_MASTER) {
        sentinelCheckObjectivelyDown(ri);
        if (sentinelStartFailoverIfNeeded(ri))
            sentinelAskMasterStateToOtherSentinels(ri, SENTINEL_ASK_FORCED);
        sentinelFailoverStateMachine(ri);
        sentinelAskMasterStateToOtherSentinels(ri, SENTINEL_NO_FLAGS);
    }
}

/* 如果ri的link是断连状态，创建一个异步的连接。
 * 注意：命令连接和订阅连接只要有一条断开，link->disconnected都会变成true */
void sentinelReconnectInstance(sentinelRedisInstance *ri) {
    if (ri->link->disconnected == 0) return;
    if (ri->addr->port == 0) return; /* port == 0 means invalid address. */
    instanceLink *link = ri->link;
    mstime_t now = mstime();

    if (now - ri->link->last_reconn_time < SENTINEL_PING_PERIOD) return;
    ri->link->last_reconn_time = now;

    /* 命令连接 */
    if (link->cc == NULL) {
        // 发起一个异步连接
        link->cc = redisAsyncConnectBind(ri->addr->ip, ri->addr->port, NET_FIRST_BIND_ADDR);
        if (link->cc->err) {
            sentinelEvent(LL_DEBUG, "-cmd-link-reconnection", ri, "%@ #%s",
                          link->cc->errstr);
            instanceLinkCloseConnection(link, link->cc);
        } else {
            link->pending_commands = 0;
            link->cc_conn_time = mstime();
            link->cc->data = link;
            // 加入事件循环，创建Connect回调，Disconnect回调，发送AUTH消息，设置Client名字，发送PING消息
            redisAeAttach(server.el, link->cc);
            redisAsyncSetConnectCallback(link->cc,
                                         sentinelLinkEstablishedCallback);
            redisAsyncSetDisconnectCallback(link->cc,
                                            sentinelDisconnectCallback);
            sentinelSendAuthIfNeeded(ri, link->cc);
            sentinelSetClientName(ri, link->cc, "cmd");

            /* 当重连后我们尽快先发送一个PING命令 */
            sentinelSendPing(ri);
        }
    }
    /* 对于master和slave，发起订阅连接 */
    if ((ri->flags & (SRI_MASTER | SRI_SLAVE)) && link->pc == NULL) {
        link->pc = redisAsyncConnectBind(ri->addr->ip, ri->addr->port, NET_FIRST_BIND_ADDR);
        if (link->pc->err) {
            sentinelEvent(LL_DEBUG, "-pubsub-link-reconnection", ri, "%@ #%s",
                          link->pc->errstr);
            instanceLinkCloseConnection(link, link->pc);
        } else {
            int retval;

            link->pc_conn_time = mstime();
            link->pc->data = link;
            // 将pc socket附加到ae事件循环框架上
            redisAeAttach(server.el, link->pc);
            redisAsyncSetConnectCallback(link->pc,
                                         sentinelLinkEstablishedCallback);
            redisAsyncSetDisconnectCallback(link->pc,
                                            sentinelDisconnectCallback);
            sentinelSendAuthIfNeeded(ri, link->pc);
            sentinelSetClientName(ri, link->pc, "pubsub");
            /* 订阅__sentinel:hello频道 */
            retval = redisAsyncCommand(link->pc,
                                       sentinelReceiveHelloMessages, ri, "%s %s",
                                       sentinelInstanceMapCommand(ri, "SUBSCRIBE"),
                                       SENTINEL_HELLO_CHANNEL);
            if (retval != C_OK) {
                /* 如果我们订阅失败，我们就关闭连接重试 */
                instanceLinkCloseConnection(link, link->pc);
                return;
            }
        }
    }
    /* 清理disconnected状态 */
    if (link->cc && (ri->flags & SRI_SENTINEL || link->pc))
        link->disconnected = 0;
}
```

#### 心跳检测
通过__sentinel__:hello频道发现其它sentinel。
```
/* 处理从master或者slave收到的__sentinel__:hello频道中的hello消息，或者从其它sentinel直接发来的信息。
 * 如果消息中指定的master name未知，消息将被丢弃。 */
void sentinelProcessHelloMessage(char *hello, int hello_len) {
    /* Format is composed of 8 tokens:
     * 0=ip,1=port,2=runid,3=current_epoch,4=master_name,
     * 5=master_ip,6=master_port,7=master_config_epoch. */
    int numtokens, port, removed, master_port;
    uint64_t current_epoch, master_config_epoch;
    char **token = sdssplitlen(hello, hello_len, ",", 1, &numtokens);
    sentinelRedisInstance *si, *master;

    if (numtokens == 8) {
        /* 包含一个master的引用 */
        master = sentinelGetMasterByName(token[4]);
        if (!master) goto cleanup; /* 未知的master，跳过 */

        /* 首先，检查我们是否有相同ip:port和runid的sentinel的信息 */
        port = atoi(token[1]);
        master_port = atoi(token[6]);
        si = getSentinelRedisInstanceByAddrAndRunID(
                master->sentinels, token[0], port, token[2]);
        current_epoch = strtoull(token[3], NULL, 10);
        master_config_epoch = strtoull(token[7], NULL, 10);

        /* Sentinel处理情况分析：
         * 1. 相同runid并且相同ip:port，什么都不必做
         * 2. 相同runid但是不同ip:port，说明这个sentinel出现了地址切换，删除，并重新添加
         * 3. 不同runid但是相同ip:port，说明这个ip:port所在的sentinel地址是非法的，我们需要标示所有具有该runid的sentinel非法，然后新增一个新的。
         * 4. 不同runid并且不同ip:port，直接新增。 */
        if (!si) {          // 如果没发现相同ip和runid的sentinel，说明这是一个新的sentinel
            /* 如果没有，因为sentinel的地址切换，我们需要移除所有相同runid的sentinels，
             * 我们将会在之后添加一个相同runid但是有新的地址的sentinel */
            removed = removeMatchingSentinelFromMaster(master, token[2]);
            if (removed) {
                // 如果找到并删除了相同runid但不同ip的sentinel，说明是sentinel进行了地址切换
                sentinelEvent(LL_NOTICE, "+sentinel-address-switch", master,
                              "%@ ip %s port %d for %s", token[0], port, token[2]);
            } else {
                /* 如果找到了相同ip:port但不同runid的sentinel，说明这个sentinel是非法的，
                 * 我们将把这个ip:port关联的sentinel标记为非法，我们将把port设置为0，来标示地址非法。
                 * 我们将会在之后收到带有该runid实例的Hello消息时更新。 */
                sentinelRedisInstance *other =
                        getSentinelRedisInstanceByAddrAndRunID(
                                master->sentinels, token[0], port, NULL);
                if (other) {
                    sentinelEvent(LL_NOTICE, "+sentinel-invalid-addr", other, "%@");
                    other->addr->port = 0; /* 这意味着：地址是非法的 */
                    sentinelUpdateSentinelAddressInAllMasters(other);
                }
            }

            /* 增加一个新的sentinel，并增加到master的sentinels中 */
            si = createSentinelRedisInstance(token[2], SRI_SENTINEL,
                                             token[0], port, master->quorum, master);

            if (si) {
                if (!removed) sentinelEvent(LL_NOTICE, "+sentinel", si, "%@");
                /* 刚创建完实例，runid为空，我们需要立即填充它，否则以后没机会了 */
                si->runid = sdsnew(token[2]);
                sentinelTryConnectionSharing(si);
                if (removed) sentinelUpdateSentinelAddressInAllMasters(si);
                sentinelFlushConfig();
            }
        }

        ...

        /* 更新sentinel的状态 */
        if (si) si->last_hello_time = mstime();
    }

    cleanup:
    sdsfreesplitres(token, numtokens);
}
```
通过master的INFO响应发现从节点
```
/* 处理从master接收到的INFO命令的回复 */
void sentinelRefreshInstanceInfo(sentinelRedisInstance *ri, const char *info) {
    sds *lines;
    int numlines, j;
    int role = 0;

    /* 缓存INFO的回复信息 */
    sdsfree(ri->info);
    ri->info = sdsnew(info);

    /* 如果在INFO输出中没有下面的域，那么需要被设定为给定的值 */
    ri->master_link_down_time = 0;

    /* 按行解析 */
    lines = sdssplitlen(info, strlen(info), "\r\n", 2, &numlines);
    for (j = 0; j < numlines; j++) {
        sentinelRedisInstance *slave;
        sds l = lines[j];

        /* 查找runid：run_id:<40 hex chars>*/
        if (sdslen(l) >= 47 && !memcmp(l, "run_id:", 7)) {
            if (ri->runid == NULL) {        // 如果为空，则填充
                ri->runid = sdsnewlen(l + 7, 40);
            } else {                        // 如果不为空，说明该实例进行过重启
                if (strncmp(ri->runid, l + 7, 40) != 0) {
                    sentinelEvent(LL_NOTICE, "+reboot", ri, "%@");
                    sdsfree(ri->runid);
                    ri->runid = sdsnewlen(l + 7, 40);
                }
            }
        }

        /* old versions: slave0:<ip>,<port>,<state>
         * new versions: slave0:ip=127.0.0.1,port=9999,... */
        /* 如果我们是从master收到的slave信息，则新增slave */
        if ((ri->flags & SRI_MASTER) &&
            sdslen(l) >= 7 &&
            !memcmp(l, "slave", 5) && isdigit(l[5])) {
            char *ip, *port, *end;

            if (strstr(l, "ip=") == NULL) {
                /* Old format. */
                ip = strchr(l, ':');
                if (!ip) continue;
                ip++; /* Now ip points to start of ip address. */
                port = strchr(ip, ',');
                if (!port) continue;
                *port = '\0'; /* nul term for easy access. */
                port++; /* Now port points to start of port number. */
                end = strchr(port, ',');
                if (!end) continue;
                *end = '\0'; /* nul term for easy access. */
            } else {
                /* New format. */
                ip = strstr(l, "ip=");
                if (!ip) continue;
                ip += 3; /* Now ip points to start of ip address. */
                port = strstr(l, "port=");
                if (!port) continue;
                port += 5; /* Now port points to start of port number. */
                /* Nul term both fields for easy access. */
                end = strchr(ip, ',');
                if (end) *end = '\0';
                end = strchr(port, ',');
                if (end) *end = '\0';
            }

            /* 如果我们没有记录这个slave，则新增 */
            if (sentinelRedisInstanceLookupSlave(ri, ip, atoi(port)) == NULL) {
                if ((slave = createSentinelRedisInstance(NULL, SRI_SLAVE, ip,
                                                         atoi(port), ri->quorum, ri)) != NULL) {
                    sentinelEvent(LL_NOTICE, "+slave", slave, "%@");
                    sentinelFlushConfig();
                }
            }
        }

        /* master_link_down_since_seconds:<seconds> */
        /* 更新主从断开时间 */
        if (sdslen(l) >= 32 &&
            !memcmp(l, "master_link_down_since_seconds", 30)) {
            ri->master_link_down_time = strtoll(l + 31, NULL, 10) * 1000;
        }

        /* role:<role> */
        if (!memcmp(l, "role:master", 11)) role = SRI_MASTER;
        else if (!memcmp(l, "role:slave", 10)) role = SRI_SLAVE;

        // 如果当前角色是SLAVE，则更新slave相关的信息
        if (role == SRI_SLAVE) {
            /* master_host:<host> */
            if (sdslen(l) >= 12 && !memcmp(l, "master_host:", 12)) {
                if (ri->slave_master_host == NULL ||
                    strcasecmp(l + 12, ri->slave_master_host)) {
                    sdsfree(ri->slave_master_host);
                    ri->slave_master_host = sdsnew(l + 12);
                    ri->slave_conf_change_time = mstime();
                }
            }

            /* master_port:<port> */
            if (sdslen(l) >= 12 && !memcmp(l, "master_port:", 12)) {
                int slave_master_port = atoi(l + 12);

                if (ri->slave_master_port != slave_master_port) {
                    ri->slave_master_port = slave_master_port;
                    ri->slave_conf_change_time = mstime();
                }
            }

            /* master_link_status:<status> */
            if (sdslen(l) >= 19 && !memcmp(l, "master_link_status:", 19)) {
                ri->slave_master_link_status =
                        (strcasecmp(l + 19, "up") == 0) ?
                        SENTINEL_MASTER_LINK_STATUS_UP :
                        SENTINEL_MASTER_LINK_STATUS_DOWN;
            }

            /* slave_priority:<priority> */
            if (sdslen(l) >= 15 && !memcmp(l, "slave_priority:", 15))
                ri->slave_priority = atoi(l + 15);

            /* slave_repl_offset:<offset> */
            if (sdslen(l) >= 18 && !memcmp(l, "slave_repl_offset:", 18))
                ri->slave_repl_offset = strtoull(l + 18, NULL, 10);
        }
    }
    ri->info_refresh = mstime();
    sdsfreesplitres(lines, numlines);

    ...
}
```

#### 故障发现
心跳探测
```
/* 发送周期性的PING、INFO和PUBLISH命令到指定的Redis实例 */
void sentinelSendPeriodicCommands(sentinelRedisInstance *ri) {
    mstime_t now = mstime();
    mstime_t info_period, ping_period;
    int retval;

    /* 如果当前没有成功连接，直接返回 */
    if (ri->link->disconnected) return;

    /* 像INFO、PING、PUBLISH这样的命令并不重要，如果这个连接上阻塞的命令过多，我们直接返回。
     * 当网络环境不好时，我们不希望发送为此消耗过多的内存。
     * 注意我们有保护措施，如果这条连接检测到超时，连接会断开并重连。 */
    if (ri->link->pending_commands >=
        SENTINEL_MAX_PENDING_COMMANDS * ri->link->refcount)
        return;

    /* 当该实例是一个处于客观下线或者故障转移中的master的slave时，INFO的发送周期从10s一次改为1s一次。
     * 这样我们可以更快的捕捉到slave向master的晋升。
     * 类似的，如果这个slave和master断连了，我们也会更频繁的监视INFO的输出，来更快的捕捉到恢复。*/
    if ((ri->flags & SRI_SLAVE) &&
        ((ri->master->flags & (SRI_O_DOWN | SRI_FAILOVER_IN_PROGRESS)) ||
         (ri->master_link_down_time != 0))) {
        info_period = 1000;
    } else {
        info_period = SENTINEL_INFO_PERIOD;
    }

    /* ping的周期为min(down_after_period,SENTINEL_PING_PERIOD) */
    ping_period = ri->down_after_period;
    if (ping_period > SENTINEL_PING_PERIOD) ping_period = SENTINEL_PING_PERIOD;

    /* 对master和slaves发送INFO命令 */
    if ((ri->flags & SRI_SENTINEL) == 0 &&
        (ri->info_refresh == 0 ||
         (now - ri->info_refresh) > info_period)) {
        retval = redisAsyncCommand(ri->link->cc,
                                   sentinelInfoReplyCallback, ri, "%s",
                                   sentinelInstanceMapCommand(ri, "INFO"));
        if (retval == C_OK) ri->link->pending_commands++;
    }

    /* 向所有实例发送PING命令 */
    if ((now - ri->link->last_pong_time) > ping_period &&
        (now - ri->link->last_ping_time) > ping_period / 2) {
        sentinelSendPing(ri);
    }

    /* 每2s向所有实例发布PUBLISH hello消息: 
     * 其中对于master和slaves通过频道发送，对sentinel通过PUBLISH发送 */
    if ((now - ri->last_pub_time) > SENTINEL_PUBLISH_PERIOD) {
        sentinelSendHello(ri);
    }
}
```
主观下线
```
/* 从我们的视角看这个节点是否是主观下线的 */
void sentinelCheckSubjectivelyDown(sentinelRedisInstance *ri) {
    mstime_t elapsed = 0;

    if (ri->link->act_ping_time)
        elapsed = mstime() - ri->link->act_ping_time;
    else if (ri->link->disconnected)
        elapsed = mstime() - ri->link->last_avail_time;

    /* 检查是否我们需要重连链接
     * 1) 如果命令连接已建立超过15s且发送了PING，但是下线周期已超过一半，还没有收到回复，就重连。
     * 2) 如果订阅连接已建立超过15s且发送了PING，但是已经过3个订阅周期=6s都没有收到回复，就重连。*/
    if (ri->link->cc &&
        (mstime() - ri->link->cc_conn_time) >
        SENTINEL_MIN_LINK_RECONNECT_PERIOD &&
        ri->link->act_ping_time != 0 && /* 有一个PING正在等待响应 */
        /* 这个阻塞的PING延迟了，我们甚至都没有收到错误信息，可能redis在执行一个很长的阻塞命令 */
        (mstime() - ri->link->act_ping_time) > (ri->down_after_period / 2) &&
        (mstime() - ri->link->last_pong_time) > (ri->down_after_period / 2)) {
        instanceLinkCloseConnection(ri->link, ri->link->cc);
    }

    if (ri->link->pc &&
        (mstime() - ri->link->pc_conn_time) >
        SENTINEL_MIN_LINK_RECONNECT_PERIOD &&
        (mstime() - ri->link->pc_last_activity) > (SENTINEL_PUBLISH_PERIOD * 3)) {
        instanceLinkCloseConnection(ri->link, ri->link->pc);
    }

    /* 如果这个实例满足以下两个条件时，我们认为它主观下线了：
     * 1) 在down_after_period时间内，没有收到PING的回复或者没有重连上。
     * 2) 我们认为这是一个master，但是他说自己是一个slave，并且已经报告了很久了。*/
    if (elapsed > ri->down_after_period ||
        (ri->flags & SRI_MASTER &&
         ri->role_reported == SRI_SLAVE &&
         mstime() - ri->role_reported_time >
         (ri->down_after_period + SENTINEL_INFO_PERIOD * 2))) {
        /* 客观下线 */
        if ((ri->flags & SRI_S_DOWN) == 0) {
            sentinelEvent(LL_WARNING, "+sdown", ri, "%@");
            ri->s_down_since_time = mstime();
            ri->flags |= SRI_S_DOWN;
        }
    } else {
        /* 客观上线 */
        if (ri->flags & SRI_S_DOWN) {
            sentinelEvent(LL_WARNING, "-sdown", ri, "%@");
            ri->flags &= ~(SRI_S_DOWN | SRI_SCRIPT_KILL_SENT);
        }
    }
}
```
客观下线
```
void sentinelAskMasterStateToOtherSentinels(sentinelRedisInstance *master, int flags) {
    dictIterator *di;
    dictEntry *de;

    di = dictGetIterator(master->sentinels);
    while ((de = dictNext(di)) != NULL) {
        sentinelRedisInstance *ri = dictGetVal(de);
        mstime_t elapsed = mstime() - ri->last_master_down_reply_time;
        char port[32];
        int retval;

        /* 如果从其他sentinel得到state的时间过长，我们认为失效了，就清理掉 */
        if (elapsed > SENTINEL_ASK_PERIOD * 5) {
            ri->flags &= ~SRI_MASTER_DOWN;
            sdsfree(ri->leader);
            ri->leader = NULL;
        }

        /* 仅当满足以下情况时，我们才发送询问消息：
         * 1) 当前master处于主观下线。
         * 2) 和sentinel是连接状态。
         * 3) 在1s内我们没有接受过sentinel is-master-down-by-addr回复信息。*/
        if ((master->flags & SRI_S_DOWN) == 0) continue;
        if (ri->link->disconnected) continue;
        if (!(flags & SENTINEL_ASK_FORCED) &&
            mstime() - ri->last_master_down_reply_time < SENTINEL_ASK_PERIOD)
            continue;

        /* 询问其他sentinel */
        ll2string(port, sizeof(port), master->addr->port);
        retval = redisAsyncCommand(ri->link->cc,
                                   sentinelReceiveIsMasterDownReply, ri,
                                   "%s is-master-down-by-addr %s %s %llu %s",
                                   sentinelInstanceMapCommand(ri, "SENTINEL"),
                                   master->addr->ip, port,
                                   sentinel.current_epoch,
                                   (master->failover_state > SENTINEL_FAILOVER_STATE_NONE) ?
                                   sentinel.myid : "*");
        if (retval == C_OK) ri->link->pending_commands++;
    }
    dictReleaseIterator(di);
}

/* 根据配置的quorum，该master是否处于客观下线状态。
 * 注意：客观下线是个法定人数，它仅仅意味着在给定的时间内有足够多的sentinels到这个实例是不可达的。
 * 然而这个消息可能会延迟，所有它不是一个强保证，不能保证：
 * N个实例在同一时刻都认为某个实例处于主观下线状态。*/
void sentinelCheckObjectivelyDown(sentinelRedisInstance *master) {
    dictIterator *di;
    dictEntry *de;
    unsigned int quorum = 0, odown = 0;

    if (master->flags & SRI_S_DOWN) {
        quorum = 1; /* 当前sentinel认为已经下线 */
        /* 统计其他节点是否认为已经下线 */
        di = dictGetIterator(master->sentinels);
        while ((de = dictNext(di)) != NULL) {
            sentinelRedisInstance *ri = dictGetVal(de);

            if (ri->flags & SRI_MASTER_DOWN) quorum++;
        }
        dictReleaseIterator(di);
        // 如果超过了预设的法定人数，则认为客观下线了
        if (quorum >= master->quorum) odown = 1;
    }

    /* 根据odown设置master状态 */
    if (odown) {
        if ((master->flags & SRI_O_DOWN) == 0) {
            sentinelEvent(LL_WARNING, "+odown", master, "%@ #quorum %d/%d",
                          quorum, master->quorum);
            master->flags |= SRI_O_DOWN;
            master->o_down_since_time = mstime();
        }
    } else {
        if (master->flags & SRI_O_DOWN) {
            sentinelEvent(LL_WARNING, "-odown", master, "%@");
            master->flags &= ~SRI_O_DOWN;
        }
    }
}

/* 这个实例用来检查是否可以开始故障转移，需要满足以下条件：
 * 1) master必须处于客观下线条件。
 * 2) 没有故障转移正在进行中。
 * 3) 故障转移冷却中：在之前的failover_timeout*2的时间内有一个故障转移开始的企图。
 * 我们还不知道我们是否能够赢得选举，所以有可能我们可是一个故障转移但是不做事情。
 * 如果故障转移开始了，我们将会返回非0。 */
int sentinelStartFailoverIfNeeded(sentinelRedisInstance *master) {
    /* 非客观下线，不开始故障转移 */
    if (!(master->flags & SRI_O_DOWN)) return 0;

    /* 故障转移进行中，不开始 */
    if (master->flags & SRI_FAILOVER_IN_PROGRESS) return 0;

    /* 故障转移，冷却中 */
    if (mstime() - master->failover_start_time <
        master->failover_timeout * 2) {
        if (master->failover_delay_logged != master->failover_start_time) {
            time_t clock = (master->failover_start_time +
                            master->failover_timeout * 2) / 1000;
            char ctimebuf[26];

            ctime_r(&clock, ctimebuf);
            ctimebuf[24] = '\0'; /* Remove newline. */
            master->failover_delay_logged = master->failover_start_time;
            serverLog(LL_WARNING,
                      "Next failover delay: I will not start a failover before %s",
                      ctimebuf);
        }
        return 0;
    }

    sentinelStartFailover(master);
    return 1;
}
```

#### 故障转移
领导选举
```
/* 设置master状态来开始一个故障转移 */
void sentinelStartFailover(sentinelRedisInstance *master) {
    serverAssert(master->flags & SRI_MASTER);

    master->failover_state = SENTINEL_FAILOVER_STATE_WAIT_START;
    master->flags |= SRI_FAILOVER_IN_PROGRESS;
    master->failover_epoch = ++sentinel.current_epoch;
    sentinelEvent(LL_WARNING, "+new-epoch", master, "%llu",
                  (unsigned long long) sentinel.current_epoch);
    sentinelEvent(LL_WARNING, "+try-failover", master, "%@");
    master->failover_start_time = mstime() + rand() % SENTINEL_MAX_DESYNC;
    master->failover_state_change_time = mstime();
}

// 发起一次投票
void sentinelHandleRedisInstance(sentinelRedisInstance *ri) {
    ...

    /* 如果当前实例是master */
    if (ri->flags & SRI_MASTER) {
        ...
        if (sentinelStartFailoverIfNeeded(ri))
            sentinelAskMasterStateToOtherSentinels(ri, SENTINEL_ASK_FORCED);
        sentinelFailoverStateMachine(ri);
        ...
    }
}

void sentinelCommand(client *c) {
    if (!strcasecmp(c->argv[1]->ptr, "masters")) {
        ...
    } else if (!strcasecmp(c->argv[1]->ptr, "is-master-down-by-addr")) {
        /* SENTINEL IS-MASTER-DOWN-BY-ADDR <ip> <port> <current-epoch> <runid>
         * 参数：
         * ip:port是要检查的master的ip和port，注意这个命令将不会通过name检查，
         * 因为理论上来说，不同的sentinel可能监控带有相同name的不同master。
         * current-epoch是为了理解我们是否被允许进行一次故障转移的投票。
         * 每一个sentinel对于一个epoch仅能投票一次。
         * runid不为空意味着我们需要为了故障转移而投票，否则将会仅进行查询。*/
        sentinelRedisInstance *ri;
        long long req_epoch;
        uint64_t leader_epoch = 0;
        char *leader = NULL;
        long port;
        int isdown = 0;

        if (c->argc != 6) goto numargserr;
        if (getLongFromObjectOrReply(c, c->argv[3], &port, NULL) != C_OK ||
            getLongLongFromObjectOrReply(c, c->argv[4], &req_epoch, NULL)
            != C_OK)
            return;
        ri = getSentinelRedisInstanceByAddrAndRunID(sentinel.masters,
                                                    c->argv[2]->ptr, port, NULL);

        /* 是否存在，是否是master，是否是主观下线状态？
         * 注意：如果我们处于TILT状态，我们总是回复0 */
        if (!sentinel.tilt && ri && (ri->flags & SRI_S_DOWN) &&
            (ri->flags & SRI_MASTER))
            isdown = 1;

        /* 为这个master投票或者拉取之前的投票结果 */
        if (ri && ri->flags & SRI_MASTER && strcasecmp(c->argv[5]->ptr, "*")) {
            leader = sentinelVoteLeader(ri, (uint64_t) req_epoch,
                                        c->argv[5]->ptr,
                                        &leader_epoch);
        }

        /* 返回回复：down state, leader, vote epoch */
        addReplyMultiBulkLen(c, 3);
        addReply(c, isdown ? shared.cone : shared.czero);
        addReplyBulkCString(c, leader ? leader : "*");
        addReplyLongLong(c, (long long) leader_epoch);
        if (leader) sdsfree(leader);
    } else if (!strcasecmp(c->argv[1]->ptr, "reset")) {
        /* SENTINEL RESET <pattern> */
        if (c->argc != 3) goto numargserr;
        addReplyLongLong(c, sentinelResetMastersByPattern(c->argv[2]->ptr, SENTINEL_GENERATE_EVENT));
    } else {
        ...
    }
}

/* 接收SENTINEL is-master-down-by-addr命令回复 */
void sentinelReceiveIsMasterDownReply(redisAsyncContext *c, void *reply, void *privdata) {
    sentinelRedisInstance *ri = privdata;
    instanceLink *link = c->data;
    redisReply *r;

    if (!reply || !link) return;
    link->pending_commands--;
    r = reply;

    /* 忽略错误或者不期望的回复。
     * 注意：如果命令回复了错误，我们将会在timeout之后清理SRI_MASTER_DOWN标志。
     * 回复格式：0: 主节点下线状态 1：runid 2: epoch */
    if (r->type == REDIS_REPLY_ARRAY && r->elements == 3 &&
        r->element[0]->type == REDIS_REPLY_INTEGER &&
        r->element[1]->type == REDIS_REPLY_STRING &&
        r->element[2]->type == REDIS_REPLY_INTEGER) {
        ri->last_master_down_reply_time = mstime();
        if (r->element[0]->integer == 1) {
            ri->flags |= SRI_MASTER_DOWN;
        } else {
            ri->flags &= ~SRI_MASTER_DOWN;
        }
        if (strcmp(r->element[1]->str, "*")) {
            /* 如果runid不是*，说明对端sentinel进行了一次投票。 */
            sdsfree(ri->leader);
            if ((long long) ri->leader_epoch != r->element[2]->integer)
                serverLog(LL_WARNING,
                          "%s voted for %s %llu", ri->name,
                          r->element[1]->str,
                          (unsigned long long) r->element[2]->integer);
            ri->leader = sdsnew(r->element[1]->str);
            ri->leader_epoch = r->element[2]->integer;
        }
    }
}
```
故障转移
```
void sentinelFailoverStateMachine(sentinelRedisInstance *ri) {
    serverAssert(ri->flags & SRI_MASTER);

    if (!(ri->flags & SRI_FAILOVER_IN_PROGRESS)) return;

    switch (ri->failover_state) {
        case SENTINEL_FAILOVER_STATE_WAIT_START:
            sentinelFailoverWaitStart(ri);
            break;
        case SENTINEL_FAILOVER_STATE_SELECT_SLAVE:
            sentinelFailoverSelectSlave(ri);
            break;
        case SENTINEL_FAILOVER_STATE_SEND_SLAVEOF_NOONE:
            sentinelFailoverSendSlaveOfNoOne(ri);
            break;
        case SENTINEL_FAILOVER_STATE_WAIT_PROMOTION:
            sentinelFailoverWaitPromotion(ri);
            break;
        case SENTINEL_FAILOVER_STATE_RECONF_SLAVES:
            sentinelFailoverReconfNextSlave(ri);
            break;
    }
}

// leader选举成功
void sentinelFailoverWaitStart(sentinelRedisInstance *ri) {
    char *leader;
    int isleader;

    /* 检查我们是否是当前故障转移纪元的leader */
    leader = sentinelGetLeader(ri, ri->failover_epoch);
    isleader = leader && strcasecmp(leader, sentinel.myid) == 0;
    sdsfree(leader);

    /* 如果我不是领导者，并且也不是通过SENTINEL FAILOVER强行开启的故障转移，那我们不能继续 */
    if (!isleader && !(ri->flags & SRI_FORCE_FAILOVER)) {
        int election_timeout = SENTINEL_ELECTION_TIMEOUT;

        /* 选举超时的时间是max(SENTINEL_ELECTION_TIMEOUT,failover_timeout) */
        if (election_timeout > ri->failover_timeout)
            election_timeout = ri->failover_timeout;
        /* 如果我在选举超时时间内都没有成为leader，则中止故障转移过程 */
        if (mstime() - ri->failover_start_time > election_timeout) {
            sentinelEvent(LL_WARNING, "-failover-abort-not-elected", ri, "%@");
            sentinelAbortFailover(ri);
        }
        return;
    }
    sentinelEvent(LL_WARNING, "+elected-leader", ri, "%@");
    if (sentinel.simfailure_flags & SENTINEL_SIMFAILURE_CRASH_AFTER_ELECTION)
        sentinelSimFailureCrash();
    ri->failover_state = SENTINEL_FAILOVER_STATE_SELECT_SLAVE;
    ri->failover_state_change_time = mstime();
    sentinelEvent(LL_WARNING, "+failover-state-select-slave", ri, "%@");
}

// 选择要晋升的slave
void sentinelFailoverSelectSlave(sentinelRedisInstance *ri) {
    sentinelRedisInstance *slave = sentinelSelectSlave(ri);

    /* 这个状态下我们不处理超时，因为这个函数会自己中止或者进入下一阶段 */
    if (slave == NULL) {
        sentinelEvent(LL_WARNING, "-failover-abort-no-good-slave", ri, "%@");
        sentinelAbortFailover(ri);
    } else {
        sentinelEvent(LL_WARNING, "+selected-slave", slave, "%@");
        slave->flags |= SRI_PROMOTED;
        ri->promoted_slave = slave;
        ri->failover_state = SENTINEL_FAILOVER_STATE_SEND_SLAVEOF_NOONE;
        ri->failover_state_change_time = mstime();
        sentinelEvent(LL_NOTICE, "+failover-state-send-slaveof-noone",
                      slave, "%@");
    }
}

/* 选择一个合适的slave来进行晋升。这个算法仅仅允许满足下列的实例：
 * 1) 不带有下列标志：S_DOWN, O_DOWN, DISCONNECTED。
 * 2) 最后收到PING回复的时间不超过5个PING周期。
 * 3) info_refresh不超过3个INFO周期。
 * 4) master_link_down_time到现在的时间不超过：
 *      (now - master->s_down_since_time) + (master->down_after_period * 10)。
 *      基本上，从我们的视角看到master下线，slave将会被断开不超过10个down-after-period。
 *      这个想法是，因为master下线了，所以slave将会堆积，但不应该堆积过久。无论如何，
 *      我们应该根据复制偏移量选择一个最好的slave。
 * 5) slave优先级不能为0，不然我们会放弃这个。
 * 满足以上条件时，我们将会按照以下条件排序：
 * - 更大的优先级
 * - 更大的复制偏移量
 * - 更小字典序的runid
 * 如果找到了合适的slave，将会返回，没找到则返回NULL */
/* sentinelSelectSlave()的辅助函数，被用于qsort()来选出"better first"的slave。 */
int compareSlavesForPromotion(const void *a, const void *b) {
    sentinelRedisInstance **sa = (sentinelRedisInstance **) a,
            **sb = (sentinelRedisInstance **) b;
    char *sa_runid, *sb_runid;

    /* 选择最大优先级 */
    if ((*sa)->slave_priority != (*sb)->slave_priority)
        return (*sa)->slave_priority - (*sb)->slave_priority;

    /* 选择最大复制量 */
    if ((*sa)->slave_repl_offset > (*sb)->slave_repl_offset) {
        return -1; /* a < b */
    } else if ((*sa)->slave_repl_offset < (*sb)->slave_repl_offset) {
        return 1; /* a > b */
    }

    /* 选择最小的runid，注意：低版本的redis不会在INFO发布runid，所以是NULL */
    sa_runid = (*sa)->runid;
    sb_runid = (*sb)->runid;
    if (sa_runid == NULL && sb_runid == NULL) return 0;
    else if (sa_runid == NULL) return 1;  /* a > b */
    else if (sb_runid == NULL) return -1; /* a < b */
    return strcasecmp(sa_runid, sb_runid);
}

sentinelRedisInstance *sentinelSelectSlave(sentinelRedisInstance *master) {
    sentinelRedisInstance **instance =
            zmalloc(sizeof(instance[0]) * dictSize(master->slaves));
    sentinelRedisInstance *selected = NULL;
    int instances = 0;
    dictIterator *di;
    dictEntry *de;
    mstime_t max_master_down_time = 0;

    if (master->flags & SRI_S_DOWN)
        max_master_down_time += mstime() - master->s_down_since_time;
    max_master_down_time += master->down_after_period * 10;

    di = dictGetIterator(master->slaves);
    while ((de = dictNext(di)) != NULL) {
        sentinelRedisInstance *slave = dictGetVal(de);
        mstime_t info_validity_time;

        if (slave->flags & (SRI_S_DOWN | SRI_O_DOWN)) continue;
        if (slave->link->disconnected) continue;
        if (mstime() - slave->link->last_avail_time > SENTINEL_PING_PERIOD * 5) continue;
        if (slave->slave_priority == 0) continue;

        /* 如果master处于SDOWN状态，我们将会从slaves每1秒获取一次INFO信息，否则是每10s一次 */
        if (master->flags & SRI_S_DOWN)
            info_validity_time = SENTINEL_PING_PERIOD * 5;
        else
            info_validity_time = SENTINEL_INFO_PERIOD * 3;
        if (mstime() - slave->info_refresh > info_validity_time) continue;
        if (slave->master_link_down_time > max_master_down_time) continue;
        instance[instances++] = slave;
    }
    dictReleaseIterator(di);
    if (instances) {
        qsort(instance, instances, sizeof(sentinelRedisInstance *),
              compareSlavesForPromotion);
        selected = instance[0];
    }
    zfree(instance);
    return selected;
}

// 提升选定的slave
void sentinelFailoverSendSlaveOfNoOne(sentinelRedisInstance *ri) {
    int retval;

    /* 如果和要晋升的slave断开了，我们无法发送命令。重试直到超时然后中止 */
    if (ri->promoted_slave->link->disconnected) {
        if (mstime() - ri->failover_state_change_time > ri->failover_timeout) {
            sentinelEvent(LL_WARNING, "-failover-abort-slave-timeout", ri, "%@");
            sentinelAbortFailover(ri);
        }
        return;
    }

    /* 发送SLAVEOF NO ONE到slave，从而把这个slave转换成一个master，
     * 我们注册了一个通用的回调，因为我们不关心回复的内容，
     * 我们将会通过不断检查INFO的返回来判断是否切换成功：slave -> master */
    retval = sentinelSendSlaveOf(ri->promoted_slave, NULL, 0);
    if (retval != C_OK) return;
    sentinelEvent(LL_NOTICE, "+failover-state-wait-promotion",
                  ri->promoted_slave, "%@");
    ri->failover_state = SENTINEL_FAILOVER_STATE_WAIT_PROMOTION;
    ri->failover_state_change_time = mstime();
}

// 等待要晋升的slave晋升完成
/* 我们将会通过一直检查INFO命令的输出来确定是否这个slave已经转变成了master */
void sentinelFailoverWaitPromotion(sentinelRedisInstance *ri) {
    /* 仅仅处理这个超时。切换到下一个状态是通过解析INFO命令的回复来确定slave的晋升的 */
    if (mstime() - ri->failover_state_change_time > ri->failover_timeout) {
        sentinelEvent(LL_WARNING, "-failover-abort-slave-timeout", ri, "%@");
        sentinelAbortFailover(ri);
    }
}

/* 处理从master接收到的INFO命令的回复 */
void sentinelRefreshInstanceInfo(sentinelRedisInstance *ri, const char *info) {
    ...

    /* ---------------------------- Acting half -----------------------------
    /* 如果处于TILT模式，则只会记录相关信息，不执行某些操作 */

    /* 当上次报告的角色和本次报告的不一样时 */
    if (role != ri->role_reported) {
        ri->role_reported_time = mstime();
        ri->role_reported = role;
        if (role == SRI_SLAVE) ri->slave_conf_change_time = mstime();
        /* 如果本次汇报的角色配置和当前配置是一致的，我们记录+role-change事件，
         * 如果本次汇报的角色配置和当前配置不一致，我们记录-role-change事件。 */
        sentinelEvent(LL_VERBOSE,
                      ((ri->flags & (SRI_MASTER | SRI_SLAVE)) == role) ?
                      "+role-change" : "-role-change",
                      ri, "%@ new reported role is %s",
                      role == SRI_MASTER ? "master" : "slave",
                      ri->flags & SRI_MASTER ? "master" : "slave");
    }

    /* 下面的行为不能在TILT模式下执行 */
    if (sentinel.tilt) return;

    /* 处理 master -> slave 角色转变 */
    if ((ri->flags & SRI_MASTER) && role == SRI_SLAVE) {
        /* 我们什么都不做，但是一个声明为slave的master被sentinel认为是无法访问的，
         * 如果该实例一直这样报告，我们将会认为它是主观下线的，最终可能会触发一次故障转移。 */
    }

    /* 处理slave->master的角色转变 */
    if ((ri->flags & SRI_SLAVE) && role == SRI_MASTER) {
        /* 如果该slave是晋升的slave，我们需要修改故障转移状态机 */
        if ((ri->flags & SRI_PROMOTED) &&
            (ri->master->flags & SRI_FAILOVER_IN_PROGRESS) &&
            (ri->master->failover_state ==
             SENTINEL_FAILOVER_STATE_WAIT_PROMOTION)) {
            /* 注意：我们确认了slave已经重新配置为了master，所以我们把master的配置纪元作为当前纪元，
             * 我们是通过这个纪元赢得故障转移的选举的。
             * 这将会强制其他sentinels更新它们的配置（假设没有一个更新的纪元可用）。 */
            ri->master->config_epoch = ri->master->failover_epoch;
            ri->master->failover_state = SENTINEL_FAILOVER_STATE_RECONF_SLAVES;
            ri->master->failover_state_change_time = mstime();
            sentinelFlushConfig();
            sentinelEvent(LL_WARNING, "+promoted-slave", ri, "%@");
            if (sentinel.simfailure_flags &
                SENTINEL_SIMFAILURE_CRASH_AFTER_PROMOTION)
                sentinelSimFailureCrash();
            sentinelEvent(LL_WARNING, "+failover-state-reconf-slaves",
                          ri->master, "%@");
            sentinelCallClientReconfScript(ri->master, SENTINEL_LEADER,
                                           "start", ri->master->addr, ri->addr);
            sentinelForceHelloUpdateForMaster(ri->master);
        } else {
            /* 另外一个slave转化为了master。我们将原来master重新配置为slave。
             * 在此之前等待8s时间，来接收新的配置，减少数据包乱序带来的影响。 */
            mstime_t wait_time = SENTINEL_PUBLISH_PERIOD * 4;

            if (!(ri->flags & SRI_PROMOTED) &&
                sentinelMasterLooksSane(ri->master) &&
                sentinelRedisInstanceNoDownFor(ri, wait_time) &&
                mstime() - ri->role_reported_time > wait_time) {
                int retval = sentinelSendSlaveOf(ri,
                                                 ri->master->addr->ip,
                                                 ri->master->addr->port);
                if (retval == C_OK)
                    sentinelEvent(LL_NOTICE, "+convert-to-slave", ri, "%@");
            }
        }
    }

    /* slaves开始跟从一个新的master */
    if ((ri->flags & SRI_SLAVE) &&
        role == SRI_SLAVE &&
        (ri->slave_master_port != ri->master->addr->port ||
         strcasecmp(ri->slave_master_host, ri->master->addr->ip))) {
        mstime_t wait_time = ri->master->failover_timeout;

        /* 在更新slave之前确保master是正常的 */
        if (sentinelMasterLooksSane(ri->master) &&
            sentinelRedisInstanceNoDownFor(ri, wait_time) &&
            mstime() - ri->slave_conf_change_time > wait_time) {
            int retval = sentinelSendSlaveOf(ri,
                                             ri->master->addr->ip,
                                             ri->master->addr->port);
            if (retval == C_OK)
                sentinelEvent(LL_NOTICE, "+fix-slave-config", ri, "%@");
        }
    }

    /* 检查slave重配置的进度状态 */
    if ((ri->flags & SRI_SLAVE) && role == SRI_SLAVE &&
        (ri->flags & (SRI_RECONF_SENT | SRI_RECONF_INPROG))) {
        /* SRI_RECONF_SENT -> SRI_RECONF_INPROG. */
        if ((ri->flags & SRI_RECONF_SENT) &&
            ri->slave_master_host &&
            strcmp(ri->slave_master_host,
                   ri->master->promoted_slave->addr->ip) == 0 &&
            ri->slave_master_port == ri->master->promoted_slave->addr->port) {
            ri->flags &= ~SRI_RECONF_SENT;
            ri->flags |= SRI_RECONF_INPROG;
            sentinelEvent(LL_NOTICE, "+slave-reconf-inprog", ri, "%@");
        }

        /* SRI_RECONF_INPROG -> SRI_RECONF_DONE */
        if ((ri->flags & SRI_RECONF_INPROG) &&
            ri->slave_master_link_status == SENTINEL_MASTER_LINK_STATUS_UP) {
            ri->flags &= ~SRI_RECONF_INPROG;
            ri->flags |= SRI_RECONF_DONE;
            sentinelEvent(LL_NOTICE, "+slave-reconf-done", ri, "%@");
        }
    }
}

// 配置其它的slaves同步新的master
/* 发送slave of <new master address>对所有未完成配置更新的slaves */
void sentinelFailoverReconfNextSlave(sentinelRedisInstance *master) {
    dictIterator *di;
    dictEntry *de;
    int in_progress = 0;

    di = dictGetIterator(master->slaves);
    while ((de = dictNext(di)) != NULL) {
        sentinelRedisInstance *slave = dictGetVal(de);

        if (slave->flags & (SRI_RECONF_SENT | SRI_RECONF_INPROG))
            in_progress++;
    }
    dictReleaseIterator(di);

    di = dictGetIterator(master->slaves);
    while (in_progress < master->parallel_syncs &&
           (de = dictNext(di)) != NULL) {
        sentinelRedisInstance *slave = dictGetVal(de);
        int retval;

        /* 跳过晋升的slave和已经完成配置的slave */
        if (slave->flags & (SRI_PROMOTED | SRI_RECONF_DONE)) continue;

        /* 如果发送SLAVEOF <new master>之后超过10s，则认为超时，我们认为它已经完成，
         * sentinels将会检查出这种情况并且在之后进行修复 */
        if ((slave->flags & SRI_RECONF_SENT) &&
            (mstime() - slave->slave_reconf_sent_time) >
            SENTINEL_SLAVE_RECONF_TIMEOUT) {
            sentinelEvent(LL_NOTICE, "-slave-reconf-sent-timeout", slave, "%@");
            slave->flags &= ~SRI_RECONF_SENT;
            slave->flags |= SRI_RECONF_DONE;
        }

        /* 对于断连的或者处于同步中的，我们直接跳过 */
        if (slave->flags & (SRI_RECONF_SENT | SRI_RECONF_INPROG)) continue;
        if (slave->link->disconnected) continue;

        /* 发送SLAVEOF <new master>命令 */
        retval = sentinelSendSlaveOf(slave,
                                     master->promoted_slave->addr->ip,
                                     master->promoted_slave->addr->port);
        if (retval == C_OK) {
            slave->flags |= SRI_RECONF_SENT;
            slave->slave_reconf_sent_time = mstime();
            sentinelEvent(LL_NOTICE, "+slave-reconf-sent", slave, "%@");
            in_progress++;
        }
    }
    dictReleaseIterator(di);

    /* 检查是否所有的slaves都已经重新配置或者处理了超时 */
    sentinelFailoverDetectEnd(master);
}

void sentinelFailoverDetectEnd(sentinelRedisInstance *master) {
    int not_reconfigured = 0, timeout = 0;
    dictIterator *di;
    dictEntry *de;
    mstime_t elapsed = mstime() - master->failover_state_change_time;

    /* 如果这个新晋升的slave不可达，我们不认为故障转移完成 */
    if (master->promoted_slave == NULL ||
        master->promoted_slave->flags & SRI_S_DOWN)
        return;

    /* 如果所有可达的slaves都已经配置好了，则故障转移就结束了 */
    di = dictGetIterator(master->slaves);
    while ((de = dictNext(di)) != NULL) {
        sentinelRedisInstance *slave = dictGetVal(de);

        if (slave->flags & (SRI_PROMOTED | SRI_RECONF_DONE)) continue;
        if (slave->flags & SRI_S_DOWN) continue;
        not_reconfigured++;
    }
    dictReleaseIterator(di);

    /* 如果故障转移超时了，我们强制结束 */
    if (elapsed > master->failover_timeout) {
        not_reconfigured = 0;
        timeout = 1;
        sentinelEvent(LL_WARNING, "+failover-end-for-timeout", master, "%@");
    }

    if (not_reconfigured == 0) {
        sentinelEvent(LL_WARNING, "+failover-end", master, "%@");
        master->failover_state = SENTINEL_FAILOVER_STATE_UPDATE_CONFIG;
        master->failover_state_change_time = mstime();
    }

    /* 如果是因为超时导致的，则向所有还没有同步master的slaves发送slaveof命令 */
    if (timeout) {
        dictIterator *di;
        dictEntry *de;

        di = dictGetIterator(master->slaves);
        while ((de = dictNext(di)) != NULL) {
            sentinelRedisInstance *slave = dictGetVal(de);
            int retval;

            if (slave->flags & (SRI_RECONF_DONE | SRI_RECONF_SENT)) continue;
            if (slave->link->disconnected) continue;

            retval = sentinelSendSlaveOf(slave,
                                         master->promoted_slave->addr->ip,
                                         master->promoted_slave->addr->port);
            if (retval == C_OK) {
                sentinelEvent(LL_NOTICE, "+slave-reconf-sent-be", slave, "%@");
                slave->flags |= SRI_RECONF_SENT;
            }
        }
        dictReleaseIterator(di);
    }
}


/* 这个函数当slave处于SENTINEL_FAILOVER_STATE_UPDATE_CONFIG状态时被调用。
 * 在这种情况下，我们将会把master从master表中移除，并把晋升的slave加入master表。 */
void sentinelFailoverSwitchToPromotedSlave(sentinelRedisInstance *master) {
    sentinelRedisInstance *ref = master->promoted_slave ?
                                 master->promoted_slave : master;

    sentinelEvent(LL_WARNING, "+switch-master", master, "%s %s %d %s %d",
                  master->name, master->addr->ip, master->addr->port,
                  ref->addr->ip, ref->addr->port);

    sentinelResetMasterAndChangeAddress(master, ref->addr->ip, ref->addr->port);
}
```

#### JedisSentinelPool实现：主备切换
- 找到任意一个可用的sentinel，通过sentinel get-master-addr-by-name获取master地址
- 通过订阅+switch-master频道，获取master地址的变更事件
- 缺点：过于依赖sentinel的+switch-master，事件丢失则无法完成客户端切换
```
  private HostAndPort initSentinels(Set<String> sentinels, final String masterName) {

    HostAndPort master = null;
    boolean sentinelAvailable = false;

    log.info("Trying to find master from available Sentinels...");

    for (String sentinel : sentinels) {
      final HostAndPort hap = HostAndPort.parseString(sentinel);

      log.debug("Connecting to Sentinel {}", hap);

      Jedis jedis = null;
      try {
        jedis = new Jedis(hap.getHost(), hap.getPort(), sentinelConnectionTimeout, sentinelSoTimeout);
        if (sentinelUser != null) {
          jedis.auth(sentinelUser, sentinelPassword);
        } else if (sentinelPassword != null) {
          jedis.auth(sentinelPassword);
        }
        if (sentinelClientName != null) {
          jedis.clientSetname(sentinelClientName);
        }

        List<String> masterAddr = jedis.sentinelGetMasterAddrByName(masterName);

        // connected to sentinel...
        sentinelAvailable = true;

        if (masterAddr == null || masterAddr.size() != 2) {
          log.warn("Can not get master addr, master name: {}. Sentinel: {}", masterName, hap);
          continue;
        }

        master = toHostAndPort(masterAddr);
        log.debug("Found Redis master at {}", master);
        break;
      } catch (JedisException e) {
        // resolves #1036, it should handle JedisException there's another chance
        // of raising JedisDataException
        log.warn(
          "Cannot get master address from sentinel running @ {}. Reason: {}. Trying next one.", hap, e);
      } finally {
        if (jedis != null) {
          jedis.close();
        }
      }
    }

    if (master == null) {
      if (sentinelAvailable) {
        // can connect to sentinel, but master name seems to not
        // monitored
        throw new JedisException("Can connect to sentinel, but " + masterName
            + " seems to be not monitored...");
      } else {
        throw new JedisConnectionException("All sentinels down, cannot determine where is "
            + masterName + " master is running...");
      }
    }

    log.info("Redis master running at {}, starting Sentinel listeners...", master);

    for (String sentinel : sentinels) {
      final HostAndPort hap = HostAndPort.parseString(sentinel);
      MasterListener masterListener = new MasterListener(masterName, hap.getHost(), hap.getPort());
      // whether MasterListener threads are alive or not, process can be stopped
      masterListener.setDaemon(true);
      masterListeners.add(masterListener);
      masterListener.start();
    }

    return master;
  }
  
    protected class MasterListener extends Thread {

    ...

    @Override
    public void run() {

      running.set(true);

      while (running.get()) {

        try {
          // double check that it is not being shutdown
          if (!running.get()) {
            break;
          }
          
          j = new Jedis(host, port, sentinelConnectionTimeout, sentinelSoTimeout);
          if (sentinelUser != null) {
            j.auth(sentinelUser, sentinelPassword);
          } else if (sentinelPassword != null) {
            j.auth(sentinelPassword);
          }
          if (sentinelClientName != null) {
            j.clientSetname(sentinelClientName);
          }

          // code for active refresh
          List<String> masterAddr = j.sentinelGetMasterAddrByName(masterName);
          if (masterAddr == null || masterAddr.size() != 2) {
            log.warn("Can not get master addr, master name: {}. Sentinel: {}:{}.", masterName, host, port);
          } else {
            initPool(toHostAndPort(masterAddr));
          }

          j.subscribe(new JedisPubSub() {
            @Override
            public void onMessage(String channel, String message) {
              log.debug("Sentinel {}:{} published: {}.", host, port, message);

              String[] switchMasterMsg = message.split(" ");

              if (switchMasterMsg.length > 3) {

                if (masterName.equals(switchMasterMsg[0])) {
                  initPool(toHostAndPort(Arrays.asList(switchMasterMsg[3], switchMasterMsg[4])));
                } else {
                  log.debug(
                    "Ignoring message on +switch-master for master name {}, our master name is {}",
                    switchMasterMsg[0], masterName);
                }

              } else {
                log.error(
                  "Invalid message received on Sentinel {}:{} on channel +switch-master: {}", host,
                  port, message);
              }
            }
          }, "+switch-master");

        } catch (JedisException e) {

          if (running.get()) {
            log.error("Lost connection to Sentinel at {}:{}. Sleeping 5000ms and retrying.", host,
              port, e);
            try {
              Thread.sleep(subscribeRetryWaitTimeMillis);
            } catch (InterruptedException e1) {
              log.error("Sleep interrupted: ", e1);
            }
          } else {
            log.debug("Unsubscribing from Sentinel at {}:{}", host, port);
          }
        } finally {
          if (j != null) {
            j.close();
          }
        }
      }
    }

    ...
  }
```

#### Redisson实现：主从切换
- 通过sentinel get-master-addr-by-name获取master地址
- 通过sentinel slaves获取slaves地址
- 通过周期性定时轮询来获取变更地址信息变更