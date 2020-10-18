## Redis Module实现

### 加载
```
void moduleCommand(client *c) {
    char *subcmd = c->argv[1]->ptr;
    if (c->argc == 2 && !strcasecmp(subcmd,"help")) {
        const char *help[] = {
"LIST -- Return a list of loaded modules.",
"LOAD <path> [arg ...] -- Load a module library from <path>.",
"UNLOAD <name> -- Unload a module.",
NULL
        };
        addReplyHelp(c, help);
    } else
    if (!strcasecmp(subcmd,"load") && c->argc >= 3) {
        robj **argv = NULL;
        int argc = 0;

        if (c->argc > 3) {
            argc = c->argc - 3;
            argv = &c->argv[3];
        }

        if (moduleLoad(c->argv[2]->ptr,(void **)argv,argc) == C_OK)
            addReply(c,shared.ok);
        else
            addReplyError(c,
                "Error loading the extension. Please check the server logs.");
    } else if (!strcasecmp(subcmd,"unload") && c->argc == 3) {
        if (moduleUnload(c->argv[2]->ptr) == C_OK)
            addReply(c,shared.ok);
        else {
            char *errmsg;
            switch(errno) {
            case ENOENT:
                errmsg = "no such module with that name";
                break;
            case EBUSY:
                errmsg = "the module exports one or more module-side data types, can't unload";
                break;
            default:
                errmsg = "operation not possible.";
                break;
            }
            addReplyErrorFormat(c,"Error unloading module: %s",errmsg);
        }
    } else if (!strcasecmp(subcmd,"list") && c->argc == 2) {
        dictIterator *di = dictGetIterator(modules);
        dictEntry *de;

        addReplyMultiBulkLen(c,dictSize(modules));
        while ((de = dictNext(di)) != NULL) {
            sds name = dictGetKey(de);
            struct RedisModule *module = dictGetVal(de);
            addReplyMultiBulkLen(c,4);
            addReplyBulkCString(c,"name");
            addReplyBulkCBuffer(c,name,sdslen(name));
            addReplyBulkCString(c,"ver");
            addReplyLongLong(c,module->ver);
        }
        dictReleaseIterator(di);
    } else {
        addReplySubcommandSyntaxError(c);
        return;
    }
}

/* 加载一个Module并且初始化它，成功返回C_OK，失败返回C_ERR。 */
int moduleLoad(const char *path, void **module_argv, int module_argc) {
    int (*onload)(void *, void **, int);
    void *handle;
    RedisModuleCtx ctx = REDISMODULE_CTX_INIT;

    handle = dlopen(path,RTLD_NOW|RTLD_LOCAL);
    if (handle == NULL) {
        serverLog(LL_WARNING, "Module %s failed to load: %s", path, dlerror());
        return C_ERR;
    }
    onload = (int (*)(void *, void **, int))(unsigned long) dlsym(handle,"RedisModule_OnLoad");
    if (onload == NULL) {
        dlclose(handle);
        serverLog(LL_WARNING,
            "Module %s does not export RedisModule_OnLoad() "
            "symbol. Module not loaded.",path);
        return C_ERR;
    }
    if (onload((void*)&ctx,module_argv,module_argc) == REDISMODULE_ERR) {
        if (ctx.module) {
            moduleUnregisterCommands(ctx.module);
            moduleFreeModuleStructure(ctx.module);
        }
        dlclose(handle);
        serverLog(LL_WARNING,
            "Module %s initialization failed. Module not loaded",path);
        return C_ERR;
    }

    /* Redis Module加载成功，注册它。 */
    dictAdd(modules,ctx.module->name,ctx.module);
    ctx.module->handle = handle;
    serverLog(LL_NOTICE,"Module '%s' loaded from %s",ctx.module->name,path);
    moduleFreeContext(&ctx);
    return C_OK;
}

/* 这个函数必须出现在每一个Redis Module中。它用来注册命令到Redis服务器中 */
int RedisModule_OnLoad(RedisModuleCtx *ctx, RedisModuleString **argv, int argc) {
    /* 注册API并检查是否有名字冲突 */
    if (RedisModule_Init(ctx,"helloworld",1,REDISMODULE_APIVER_1)
        == REDISMODULE_ERR) return REDISMODULE_ERR;

    /* 记录加载Module时的参数列表 */
    for (int j = 0; j < argc; j++) {
        const char *s = RedisModule_StringPtrLen(argv[j],NULL);
        printf("Module loaded with ARGV[%d] = %s\n", j, s);
    }

    if (RedisModule_CreateCommand(ctx,"hello.simple",
        HelloSimple_RedisCommand,"readonly",0,0,0) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.push.native",
        HelloPushNative_RedisCommand,"write deny-oom",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.push.call",
        HelloPushCall_RedisCommand,"write deny-oom",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.push.call2",
        HelloPushCall2_RedisCommand,"write deny-oom",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.list.sum.len",
        HelloListSumLen_RedisCommand,"readonly",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.list.splice",
        HelloListSplice_RedisCommand,"write deny-oom",1,2,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.list.splice.auto",
        HelloListSpliceAuto_RedisCommand,
        "write deny-oom",1,2,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.rand.array",
        HelloRandArray_RedisCommand,"readonly",0,0,0) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.repl1",
        HelloRepl1_RedisCommand,"write",0,0,0) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.repl2",
        HelloRepl2_RedisCommand,"write",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.toggle.case",
        HelloToggleCase_RedisCommand,"write",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.more.expire",
        HelloMoreExpire_RedisCommand,"write",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.zsumrange",
        HelloZsumRange_RedisCommand,"readonly",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.lexrange",
        HelloLexRange_RedisCommand,"readonly",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.hcopy",
        HelloHCopy_RedisCommand,"write deny-oom",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    if (RedisModule_CreateCommand(ctx,"hello.leftpad",
        HelloLeftPad_RedisCommand,"",1,1,1) == REDISMODULE_ERR)
        return REDISMODULE_ERR;

    return REDISMODULE_OK;
}

/* 注册一个新的命令到Redis服务器，将会通过Redis Module协议来处理对应的请求。
 * 如果命令名已经存在或者标识位出现错误将会返回C_ERR，否则返回C_OK。
 * 这个函数必须在RedisModule_OnLoad()中调用，在初始化函数之外调用的行为是未定义的。
 * 命令函数的形式应如下所示：
 * int MyCommand_RedisCommand(RedisModuleCtx *ctx, RedisModuleString **argv, int argc)
 *
 * "strflags"是一个集合，用来指定该命令的行为，形式如"write deny-oom"
 * write： 命令可能会修改数据集。
 * readonly： 命令是只读的。
 * admin： 管理员命令，可能会改变复制或其它类似的任务。
 * deny-oom： 可能会增加内存，受内存淘汰策略影响。
 * deny-script： 不允许在Lua脚本中执行。
 * allow-loading： 允许在加载数据集时执行，仅在命令不与数据集交互式执行。
 * pubsub： 这个命令会通过Pub/Sub发布信息。
 * random： 该命令受随机化影响。
 * allow-stale： 该命令可以在从服务器用陈旧的数据响应给客户端。
 * no-monitor： 该命令的参数中有敏感信息，不要传播给monitor。
 * fast： 该命令的时间复杂度不大于O(logN)。
 * getkeys-api： 该命令实现了用来返回keys的接口，使用first/last/step无法描述key。
 * no-cluster： 该命令不应该在Redis Cluster中注册，可能是不支持报告keys的位置，
 *              以编程方式创建key名，或者其它原因。
 */
int RM_CreateCommand(RedisModuleCtx *ctx, const char *name, RedisModuleCmdFunc cmdfunc, const char *strflags, int firstkey, int lastkey, int keystep) {
    int flags = strflags ? commandFlagsFromString((char*)strflags) : 0;
    if (flags == -1) return REDISMODULE_ERR;
    if ((flags & CMD_MODULE_NO_CLUSTER) && server.cluster_enabled)
        return REDISMODULE_ERR;

    struct redisCommand *rediscmd;
    RedisModuleCommandProxy *cp;
    sds cmdname = sdsnew(name);

    /* 检查是否有重名的命令 */
    if (lookupCommand(cmdname) != NULL) {
        sdsfree(cmdname);
        return REDISMODULE_ERR;
    }

    /* 创建一个命令代理，绑定到一个正常的Redis命令上，添加到命令表中，
     * 这样我们可以从这个正常的Redis命令找到原Module。
     * 注意：我们使用Redis的getkeys_proc来保存cp结构 */
    cp = zmalloc(sizeof(*cp));
    cp->module = ctx->module;
    cp->func = cmdfunc;
    cp->rediscmd = zmalloc(sizeof(*rediscmd));
    cp->rediscmd->name = cmdname;
    cp->rediscmd->proc = RedisModuleCommandDispatcher;
    cp->rediscmd->arity = -1;
    cp->rediscmd->flags = flags | CMD_MODULE;
    cp->rediscmd->getkeys_proc = (redisGetKeysProc*)(unsigned long)cp;
    cp->rediscmd->firstkey = firstkey;
    cp->rediscmd->lastkey = lastkey;
    cp->rediscmd->keystep = keystep;
    cp->rediscmd->microseconds = 0;
    cp->rediscmd->calls = 0;
    dictAdd(server.commands,sdsdup(cmdname),cp->rediscmd);
    dictAdd(server.orig_commands,sdsdup(cmdname),cp->rediscmd);
    return REDISMODULE_OK;
}

/* 将这个从module中导出的命令命令绑定到一个正常的Redis命令上。 */
void RedisModuleCommandDispatcher(client *c) {
    RedisModuleCommandProxy *cp = (void*)(unsigned long)c->cmd->getkeys_proc;
    RedisModuleCtx ctx = REDISMODULE_CTX_INIT;

    ctx.module = cp->module;
    ctx.client = c;
    cp->func(&ctx,(void**)c->argv,c->argc);
    moduleHandlePropagationAfterCommandCallback(&ctx);
    moduleFreeContext(&ctx);
}
```

### 卸载
```
/* 卸载使用指定名字注册的module。成功时返回C_OK，否则返回C_ERR，并设置error。
 * ENONET：没有指定名称的模块。
 * EBUSY：这个模块导出了新的数据类型，仅仅可以重新加载。*/
int moduleUnload(sds name) {
    struct RedisModule *module = dictFetchValue(modules,name);

    if (module == NULL) {
        errno = ENOENT;
        return REDISMODULE_ERR;
    }

    if (listLength(module->types)) {
        errno = EBUSY;
        return REDISMODULE_ERR;
    }

    moduleUnregisterCommands(module);

    /* Remove any notification subscribers this module might have */
    moduleUnsubscribeNotifications(module);

    /* 注销所有的hooks. TODO: Yet no hooks support here. */

    /* 卸载所有的动态连接库 */
    if (dlclose(module->handle) == -1) {
        char *error = dlerror();
        if (error == NULL) error = "Unknown error";
        serverLog(LL_WARNING,"Error when trying to close the %s module: %s",
            module->name, error);
    }

    /* 从modules中移除module */
    serverLog(LL_NOTICE,"Module %s unloaded",module->name);
    dictDelete(modules,module->name);
    module->name = NULL; /* 这个name已经被dictDelete()释放 */
    moduleFreeModuleStructure(module);

    return REDISMODULE_OK;
}
```

