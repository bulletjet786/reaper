## InnoDB加锁分析

在事务的并发控制，MySQL使用MVCC来支持快照读和使用加锁来支持锁定读两种方式，锁定通过行锁和间隙锁。

锁定表：
 . | RU | RC | RR | S+
---|---|---|---|---
select | 读最新 | RV:每次都生成 | RV:第一次时生成 | 转为selectS
selectS | recordLock | recordLock | recordLock+gapLock | recordLock+gapLock
selectX | recordLock | recordLock | recordLock+gapLock | recordLock+gapLock
delete | recordLock+H | recordLock+H | recordLock+gapLock+H | recordLock+gapLock
update | recordLock+H | recordLock+H | recordLock+gapLock+H | recordLock+gapLock
insert | H | H | H | H

锁定规则：
1. MySQL的行锁(包括recordLock, gapLock, nextKeyLock)是锁定在索引上的，加锁是在使用索引扫描数据时加的。
2. 如果扫描的是聚簇索引，则直接在聚簇索引上加record+gap锁，如果扫描的是二级索引，则先在二级索引上加record+gap锁，然后在聚簇索引上加record锁。
3. 当扫描索引时，在RR级别会加recordLock和gapLock，在RU/RC级别只会加recordLock，recordLock用于锁定对已存在的记录的读取和写入，gapLock用于锁定对索引区间的插入。
4. 记录的前一gap锁和本条记录的record会合并成nextKeyLock。
5. 如果是唯一索引，且任一侧查询条件的边界匹配到了记录，可将该侧的gap锁去除。
6. mysql可能通过讲gap锁升级nextKey锁来减少锁的数量。
7. 同一个查询如果使用不同的索引，可能锁定的范围不通！
8. 对于delete，where条件中的加锁和selectX一致，对于所有的二级索引，加隐式锁。
9. 对于update，where条件中的加锁和selectX一致，对于涉及到的二级索引，加隐式锁。
10. 对于insert，加隐式锁。
11. 隐式锁: 通过比较索引记录的trx_id是否是当前活跃的事务，如果是，则说明此时有事务(记为1)正在写该记录，当其他事务(记为2)想要获取该记录的锁时，则先为事务1获取X锁，再为事务2获取对应的锁且等待。

## 加锁流程
### 测试数据
``` sql
CREATE TABLE `test` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(32) NOT NULL DEFAULT '',
  `country` int(10) unsigned NOT NULL DEFAULT '0',
  `status` int(11) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_name` (`name`),
  KEY `idx_country` (`country`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

数据：
INSERT INTO `test` (`id`, `name`, `country`)
VALUES
	(1, 'a', 1, 1),
	(3, 'c', 3, 1),
	(5, 'e', 5, 0),
	(7, 'g', 5, 0),
	(9, 'i', 7, 0);

索引：
id: 1,3,5,7,9
name: a,c,e,g,i
country: 1,3,5,5,7
```

### 无脑分析MySQL锁定范围
步骤0-8为二级索引的全步骤加锁分析，9为聚簇索引的加锁分析，10为update语句的加锁分析，11为delete语句的加锁分析。

0. 准备表格，填入待分析语句
```
分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：
使用索引：
索引排列：
    二级索引：
    聚簇索引：
二级索引：
    gap:
    record:
    最终lock:
聚簇索引：
    gap:
    record:
    最终lock:
```

1. 检查隔离级别
``` sql
mysql> select @@transaction_isolation;
+-------------------------+
| @@transaction_isolation |
+-------------------------+
| REPEATABLE-READ         |
+-------------------------+

分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：REPEATABLE-READ
扫描索引：
索引排列：
    二级索引：
    聚簇索引：
二级索引：
    gap:
    record:
    最终lock:
聚簇索引：
    gap:
    record:
    最终lock:
```

2. 通过使用explain或者optimizer_trace查看使用的索引，使用show create tabel检查索引类型；
```
mysql> explain select * from test where `name`>"c" and `name`<="g" for update;
+----+-------------+-------+------------+-------+---------------+----------+---------+------+------+----------+-----------------------+
| id | select_type | table | partitions | type  | possible_keys | key      | key_len | ref  | rows | filtered | Extra                 |
+----+-------------+-------+------------+-------+---------------+----------+---------+------+------+----------+-----------------------+
|  1 | SIMPLE      | test  | NULL       | range | idx_name      | idx_name | 98      | NULL |    2 |   100.00 | Using index condition |
+----+-------------+-------+------------+-------+---------------+----------+---------+------+------+----------+-----------------------+

分析语句：select * from test where `name`="e" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：
    聚簇索引：
二级索引：
    gap：
    record:
    最终lock:
聚簇索引：
    gap：
    record:
    最终lock:
```

3. 通过构造索引排列，可以帮助我们更好的理解锁定范围，扫描的索引是二级非唯一索引，()表示间隙
```
分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () e () g () i ()
    聚簇索引：() 1 () 3 () 5 () 7 () 9 ()
二级索引：
    gap:
    record:
    最终lock:
聚簇索引：
    gap:
    record:
    最终lock:
```

4. 确定二级索引扫描到的记录范围，使用''表示
```
分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () 'e () g' () i ()
    聚簇索引：() 1 () 3 () 5 () 7 () 9 ()
二级索引：
    gap: 
    record: 
    最终lock: 
聚簇索引：
    gap:
    record:
    最终lock:
```

5. 确定二级索引锁定范围:
    - record锁：
        - RU或者RC级别，则在''中间的所有匹配的记录加recordLock，即记录e,g
        - RR级别，则在''中间的所有记录加recordLock，即记录e,g
    - gap锁：
        - RU或者RC级别，不用填写
        - RR级别，如果是二级非唯一索引，在''中间及两侧的所有记录加gap锁，即e两侧的间隙，我们使用(c,e)和(e,g)表示，如果是唯一索引，且任一侧查询条件的边界匹配到了记录，可将该侧的gap锁去除
```
分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () 'e () g' () i ()
    聚簇索引：() 1 () 3 () 5 () 7 () 9 ()
二级索引：
    gap: (c,e),(e,g),(g,i)
    record: e,g
    最终lock: 
聚簇索引:
    gap:
    record:
    最终lock: 
```

6. 确定聚簇索引锁定范围：
    - 找到二级索引中的锁定的所有记录，对聚簇索引中的响应记录加record锁
```
分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () 'e () g' () i ()
    聚簇索引：() 1 () 3 () 5 () 7 () 9 ()
二级索引：
    gap: (c,e),(e,g),(g,i)
    record: e,g
    最终lock: 
聚簇索引:
    gap:
    record: 5,7
    最终lock: 
```

7. 合并锁区间
    - 将record锁和gap进行合并，使用(c,e]表示
    - 同一事务同一个页同一类型同一状态的锁可以被存储在同一个内存结构中从而节约存储空间，mysql可能通过讲gap锁升级nextKey锁来减少锁的数量，会扩大锁定范围，但是可以节约空间
```
分析语句：select * from test where `name`>"c" and `name`<="g" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () 'e () g' () i ()
    聚簇索引：() 1 () 3 () 5 () 7 () 9 ()
二级索引：
    gap: (c,e),(e,g),(g,i)
    record: e,g
    最终lock: (c,e],(e,g],(g,i) -> (c,e],(e,g],(g,i]
聚簇索引:
    gap:
    record: 5,7
    最终lock: 5,7
```

8. 验证：8.0以后可以通过performance_schema.data_locks查看锁定情况
```
+--------+-----------------------------------------+-----------------------+-----------+----------+---------------+-------------+----------------+-------------------+------------+-----------------------+-----------+---------------+-------------+-----------+
| ENGINE | ENGINE_LOCK_ID                          | ENGINE_TRANSACTION_ID | THREAD_ID | EVENT_ID | OBJECT_SCHEMA | OBJECT_NAME | PARTITION_NAME | SUBPARTITION_NAME | INDEX_NAME | OBJECT_INSTANCE_BEGIN | LOCK_TYPE | LOCK_MODE     | LOCK_STATUS | LOCK_DATA |
+--------+-----------------------------------------+-----------------------+-----------+----------+---------------+-------------+----------------+-------------------+------------+-----------------------+-----------+---------------+-------------+-----------+
| INNODB | 140312364539208:1208:140312627602512    |                 76590 |        58 |       58 | expert        | test        | NULL           | NULL              | NULL       |       140312627602512 | TABLE     | IX            | GRANTED     | NULL      |
| INNODB | 140312364539208:226:5:4:140312639144480 |                 76590 |        58 |       58 | expert        | test        | NULL           | NULL              | idx_name   |       140312639144480 | RECORD    | X             | GRANTED     | 'e', 5    |
| INNODB | 140312364539208:226:5:5:140312639144480 |                 76590 |        58 |       58 | expert        | test        | NULL           | NULL              | idx_name   |       140312639144480 | RECORD    | X             | GRANTED     | 'g', 7    |
| INNODB | 140312364539208:226:5:6:140312639144480 |                 76590 |        58 |       58 | expert        | test        | NULL           | NULL              | idx_name   |       140312639144480 | RECORD    | X             | GRANTED     | 'i', 9    |
| INNODB | 140312364539208:226:4:4:140312639144824 |                 76590 |        58 |       58 | expert        | test        | NULL           | NULL              | PRIMARY    |       140312639144824 | RECORD    | X,REC_NOT_GAP | GRANTED     | 5         |
| INNODB | 140312364539208:226:4:5:140312639144824 |                 76590 |        58 |       58 | expert        | test        | NULL           | NULL              | PRIMARY    |       140312639144824 | RECORD    | X,REC_NOT_GAP | GRANTED     | 7         |
+--------+-----------------------------------------+-----------------------+-----------+----------+---------------+-------------+----------------+-------------------+------------+-----------------------+-----------+---------------+-------------+-----------+    
```

9. 聚簇索引：扫描索引为聚簇索引，则该索引的分析同二级唯一索引
    - 确定扫描范围
    - 为扫描范围中的记录加gap锁和record锁，同二级唯一索引
    - 将record锁和gap进行合并
```
分析语句：select * from test where `id`>=3 and `id`<6 for update;
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () '3 () 5 ( ' ) 7 () 9 ()
二级索引:
    gap锁:
    record锁:
    最终lock: 
聚簇索引：
    gap锁:  (3,5), (5,7)
    record锁: 3，5
    最终lock: 3, (3,5], (5,7)
```

10. 删除语句：显式锁同selectX，在所有二级索引上加隐式锁
```
分析语句：delete from test where `id`>=3 and `id`<6;
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () '3 () 5 ( ' ) 7 () 9 ()
二级索引:
    gap锁:
    record锁:
    最终lock: 
聚簇索引：
    gap锁:  (3,5), (5,7)
    record锁: 3，5
    最终lock: 3, (3,5], (5,7)
隐式：
    idx_name: c,e
    idx_country: 3,5
```

11. 更新语句：显式锁同selectX，在涉及的二级索引上加隐式锁
```
分析语句：update test set `name`="t" where `id`>=3 and `id`<6;
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () '3 () 5 ( ' ) 7 () 9 ()
二级索引:
    gap锁:
    record锁:
    最终lock: 
聚簇索引：
    gap锁:  (3,5), (5,7)
    record锁: 3，5
    最终lock: 3, (3,5], (5,7)
隐式：
    idx_name隐式: c,e,t
```

12. 插入语句：显式锁同selectX，在所有二级索引上加隐式锁
```
分析语句：insert test value(4,"d", 1, 0);
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () '3 () 4 () 5 ( ' ) 7 () 9 ()
二级索引:
    gap锁:
    record锁:
    最终lock: 
聚簇索引：
    gap锁:
    record锁:
    最终lock:
隐式：
    idx_name: d
    idx_country: 1
```

### 更多例子
1. 使用主键锁定一个已存在的记录: 
```
分析语句：select * from test where `id`=5 for update;
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () 3 () '5' () 7 () 9 ()
二级索引：
    gap锁：
    record锁: 
    最终lock：
聚簇索引：
    gap锁：
    record锁：5
    最终lock：5
```

2. 使用主键锁定一个不存在的记录
```
分析语句：select * from test where `id`=4 for update;
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () 3 ( '' ) 5 () 7 () 9 ()
二级索引：
    gap锁：
    record锁: 
    最终lock：
聚簇索引：
    gap锁：(3,5)
    record锁：
    最终lock：(3,5)
```

3. 使用主键锁定范围并条件过滤：
为什么在RR级别下id=5的记录加了record锁，但是在RC级别下没有加record锁？
因为此时如果不对id=5的记录加record锁，则其他事务可能通过update语句将id=5的status修改为1，当前事务再次读取将会产生幻读。
```
分析语句：select * from test where `id`>=3 and `id`<6 and `status`= 1 for update;
隔离级别：REPEATABLE-READ
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () '3 () 5 ( ' ) 7 () 9 ()
二级索引：
    gap锁：
    record锁: 
聚簇索引：
    gap锁：(1,3), (3,5), (5,7)
    record锁：3，5
    最终lock：3, (3,5], (5,7) 
    
分析语句：select * from test where `id`>=3 and `id`<6 and `status`= 1 for update;
隔离级别：READ-COMMITTED
扫描索引：聚簇索引
索引排列：
    二级索引：
    聚簇索引：() 1 () '3 () 5 ( ' ) 7 () 9 ()
二级索引：
    gap锁：
    record锁: 
聚簇索引：
    gap锁：
    record锁：3
    最终lock：3 
```

4. 使用二级索引扫描无数据的区间：
```
分析语句：select * from test where `name`="f" for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () e ('') g () i ()
    聚簇索引：() 1 () 3 () 5 (  ) 7 () 9 ()
二级索引:
    gap锁: (e,g)
    record锁: 
    最终lock: (e,g)
聚簇索引:
    gap锁:
    record锁:
    最终lock:
```

5. 使用二级索引扫描一个范围并过滤部分记录：
```
分析语句：select * from test where `name`>"c" and `name`<="g" and status=0 for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c ( ' ) e () g' () i ()
    聚簇索引：() 1 () 3  ()   5 () 7  () 9 ()
二级索引：
    gap锁：(c,e),(e,g),(g,i)
    record锁: e,g
    最终lock：
聚簇索引：
    gap锁：
    record锁：3,5
    最终lock：3,5
```

6. 相同的条件，不同的索引，不同的锁定范围
```
分析语句：select * from test force index(`idx_name`) where `name`="e" and `country`=5 for update;
隔离级别：REPEATABLE-READ
扫描索引：idx_name/二级非唯一索引
索引排列：
    二级索引：() a () c () 'e' () g () i ()
    聚簇索引：() 1 () 3 ()  5  () 7  () 9 ()
二级索引：
    gap锁：(c,e),(e,g)
    record锁: e
    最终lock：(c,e],(e,g)
聚簇索引：
    gap锁：
    record锁：5
    最终lock：5

分析语句：select * from test force index(`idx_country`) where `name`="e" and `country`=5 for update;;
隔离级别：REPEATABLE-READ
扫描索引：idx_country/二级非唯一索引
索引排列：
    二级索引：() 1 () 3 () '5 () 5' () 7 ()
    聚簇索引：() 1 () 3 ()  5 () 7  () 9 ()
二级索引：
    gap锁：(3,5),(5,5),(5,7)
    record锁: 5,5
    最终lock：(3,5],(5,5],(5,7)
聚簇索引：
    gap锁：
    record锁：5,7
    最终lock：5,7
```

## 锁的内存结构
以下条件相同的公用同一个锁结构：
- 在同一个事务中进行加锁操作
- 被加锁的记录在同一个页面中
- 加锁的类型是一样的
- 等待状态是一样的
```
type Lock struct {
    TxInfo // 事务信息
    IndexInfo // 索引信息
    TableOrRecordInfo // 表锁信息或者行锁信息
    TypeMode // 锁类型
    Others   // 其他信息
    Bits
}

type RecordInfo struct {
    SpaceID int // 表空间
    PageNumber int // 页号
    Nbits // Bits占用位数，每个索引记录占用1位
}

type TypeMode struct {
    recLockType [24]bit // 仅行锁时有意义，nextKey锁，record锁，gap锁，插入意向锁 ... 是否正在等待
    lockType [4]bit // 表锁、行锁
    lockMode [4]bit // 0:IS 1:IX 2:S 3:X 4:AutoInc
}
```

## 该如何理解Mysql加锁的逻辑

### 数据结构
我们从数据结构的角度来理解MySQL，则MySQL的数据结构如图所示
- 一个表是由多个索引组成的，每一个索引都是一个有序链表
- 每一个索引有两种类型的节点组成，分别是Gap节点和Record节点
- 对于Record类型的节点，有三种锁定状态，未锁，X锁和S锁，对应为读写
- 对于Gap类型的节点，也有三种锁定状态，未锁，X锁和S锁，对应为读写，只有获取到了X锁才能对节点进行操作
- MySQL将Gap节点和Record节点放在了一起来进行表示，每一个Gap节点被合并到了后继的Record节点中来进行表示。
- 对于这个数据结构，MySQL提供了在扫描时的多种锁定范围：
    - 锁定整个数据结构，即表锁定，通过表锁实现
    - 锁定一个索引中的多行记录，即行锁定，通过Record锁实现，用于实现RU/RC的锁定读。
    - 锁定一个索引中的一个范围，通过Record锁和Gap锁实现，用于实现RR/S+的锁定读。
![InnoLock](https://tva1.sinaimg.cn/large/008eGmZEly1gn9d2lin1pj30rj06fjr9.jpg)

### 操作
#### Select操作
- 如果是对直接对主索引进行扫描，则锁只需要加在主索引上
- 如果需要对二级索引加锁，则还要为对对应的主索引记录加Record锁。

#### Insert操作
当插入一个节点时，需要在主索引记录和所有二级索引记录上插入该数据：
- 获取新插入的数据的X锁
- 获取插入位置所在的Gap的X锁，然后将一个Gap分裂为两个Gap，并获取这两个Gap的X锁

但是，MySQL取了个巧，这个技巧叫做隐式锁，可以减少加锁的操作。
- 在插入时，不获取任何锁
- 在其他事务扫描时，判断这个记录是否正在被其他是否修改（索引记录的trx_id对应的事务是否正在活跃），如果是，则这时候再为插入记录的事务获取应有的X锁。

#### Delete操作
当删除一个节点时，需要在主索引记录和所有二级索引记录上删除该数据：
- 获取删除的数据的X锁
- 获取待删除数据周围的两个Gap的X锁，然后将两个Gap合并为一个Gap，并获取X锁

但是，MySQL仍然可以使用隐式锁。

#### Update操作
当更新一个节点时，需要在主索引记录和所有涉及到的二级索引记录上更新该数据：
如果新的记录更新后存储空间和位置都不变，则可以进行原地更新：
- 获取更新的记录的X锁
否则，则
- 将原记录进行Delete，并Insert一条新的记录