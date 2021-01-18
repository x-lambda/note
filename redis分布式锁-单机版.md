# Redis分布式锁-单机版

1. 客户端为了获取锁，需要向`Redis`节点发送`SET`命令
```shell
SET lock_key random_value NX PX TTL
```

如果上面的命令执行成功，则客户端成功获取锁，可以执行同步操作，如果上面命令执行失败，说明获取锁失败。
注意
	* `random_value`: 是为了保证一个客户端在一段时间内获取锁的请求都是唯一的
	* `NX`: 只有建不存在的时候才操作
	* `PX TTL`: 根据业务需求选择合适的时间，例如一次请求最大超时时间`300ms`，`key`对应的`TTL`可以设置`>=300ms`

最后客户端完成了同步操作，执行`redis lua`脚本来释放锁
```shell
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
```

2. 这个锁为什么一定要设置过期时间，而且不宜过长
如果不设过期时间，那么客户端崩溃之后，这个锁永远不会被释放。
这个过期时间也