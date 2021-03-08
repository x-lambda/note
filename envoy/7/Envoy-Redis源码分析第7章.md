# `Envoy-Redis`源码分析 第7章

### 序

前一章我们说到了连接池，最后只贴出了`ConnPool::InstanceImpl`定义。眼看距离转发就一步之遥了，今天我们

就把剩下的流程走完。

还是接前面的调用

```c++
// /envoy/source/extensions/filters/network/redis_proxy/conn_pool_impl.cc
// This method is always called from a InstanceSharedPtr we don't have to worry about tls_->getTyped
// failing due to InstanceImpl going away.
Common::Redis::Client::PoolRequest*
InstanceImpl::makeRequest(const std::string& key, RespVariant&& request, PoolCallbacks& callbacks) {
  // 这个 ThreadLocalPool 就是 struct ConnPool::InstanceImpl::ThreadLocalPool{}
  return tls_->getTyped<ThreadLocalPool>().makeRequest(key, std::move(request), callbacks);
}
```

可知，继续看

```c++
// /envoy/source/extensions/filters/network/redis_proxy/conn_pool_impl.cc
Common::Redis::Client::PoolRequest*
InstanceImpl::ThreadLocalPool::makeRequest(const std::string& key, RespVariant&& request,
                                           PoolCallbacks& callbacks) {
  if (cluster_ == nullptr) {
    ASSERT(client_map_.empty());
    ASSERT(host_set_member_update_cb_handle_ == nullptr);
    return nullptr;
  }

  // lb_context 可以根据key计算出来一个hash值
  Clusters::Redis::RedisLoadBalancerContextImpl lb_context(key, 
      config_->enableHashtagging(), is_redis_cluster_, getRequest(request),
      config_->readPolicy());
  
  // 根据负载均衡策略从上游选择一个host
  Upstream::HostConstSharedPtr host = cluster_->loadBalancer().chooseHost(&lb_context);
  if (!host) {
    return nullptr;
  }
  pending_requests_.emplace_back(*this, std::move(request), callbacks);
  PendingRequest& pending_request = pending_requests_.back();
  
  // 重点在这 
  ThreadLocalActiveClientPtr& client = this->threadLocalActiveClient(host);
  pending_request.request_handler_ = client->redis_client_->makeRequest(
      getRequest(pending_request.incoming_request_), pending_request);
  if (pending_request.request_handler_) {
    return &pending_request;
  } else {
    onRequestCompleted();
    return nullptr;
  }
}
```

我们看看这个`client`是怎么来的，首先还是得从`ConnPool::InstanceImpl::ThreadLocalPool`说起

```c++
// /envoy/source/extensions/filters/network/redis_proxy/conn_pool_impl.h
struct ThreadLocalPool : public ThreadLocal::ThreadLocalObject,
                           public Upstream::ClusterUpdateCallbacks {
  ThreadLocalPool(std::shared_ptr<InstanceImpl> parent, Event::Dispatcher& dispatcher,
    std::string cluster_name);
  ~ThreadLocalPool() override;
                             
  ThreadLocalActiveClientPtr& threadLocalActiveClient(Upstream::HostConstSharedPtr host);
    
  Common::Redis::Client::PoolRequest* makeRequest(const std::string& key, 
      RespVariant&& request, PoolCallbacks& callbacks);
    
  Common::Redis::Client::PoolRequest* makeRequestToHost(const std::string& host_address,
    const Common::Redis::RespValue& request, Common::Redis::Client::ClientCallbacks& callbacks);

  absl::node_hash_map<Upstream::HostConstSharedPtr, ThreadLocalActiveClientPtr> client_map_;
  absl::node_hash_map<std::string, Upstream::HostConstSharedPtr> host_address_map_;
  std::list<Upstream::HostSharedPtr> created_via_redirect_hosts_;
  std::list<ThreadLocalActiveClientPtr> clients_to_drain_;
  std::list<PendingRequest> pending_requests_;

    /* This timer is used to poll the active clients in clients_to_drain_ to determine whether they
     * have been drained (have no active requests) or not. It is only enabled after a client has
     * been added to clients_to_drain_, and is only re-enabled as long as that list is not empty. A
     * timer is being used as opposed to using a callback to avoid adding a check of
     * clients_to_drain_ to the main data code path as this should only rarely be not empty.
     */
    Event::TimerPtr drain_timer_;
    bool is_redis_cluster_;
    Common::Redis::Client::ClientFactory& client_factory_;
    Common::Redis::Client::ConfigSharedPtr config_;
    Stats::ScopeSharedPtr stats_scope_;
    Common::Redis::RedisCommandStatsSharedPtr redis_command_stats_;
    RedisClusterStats redis_cluster_stats_;
};
```

为了简单说明，我把里面不是很重要的方法和对象都删了，我们挑核心的说。

```c++
// /envoy/source/extensions/filters/network/redis_proxy/conn_pool_impl.cc
InstanceImpl::ThreadLocalActiveClientPtr&
InstanceImpl::ThreadLocalPool::threadLocalActiveClient(Upstream::HostConstSharedPtr host) {
  // 首先在 client_map_ 中寻找有没有对应的的host上的连接
  ThreadLocalActiveClientPtr& client = client_map_[host];
  if (!client) {
    // 如果没有就创建一个新的连接
    client = std::make_unique<ThreadLocalActiveClient>(*this);
    client->host_ = host;
    client->redis_client_ = client_factory_.create(host, dispatcher_, *config_, 
      redis_command_stats_, *(stats_scope_), auth_username_, auth_password_);
    client->redis_client_->addConnectionCallbacks(*client);
  }
  
  return client;
}
```

至此目光转移到`client`这，看看创建`client`的过程

```c++
// /envoy/source/extensions/filters/network/common/redis/client_impl.cc
ClientPtr ClientFactoryImpl::create(Upstream::HostConstSharedPtr host,
                                    Event::Dispatcher& dispatcher, const Config& config,
                                    const RedisCommandStatsSharedPtr& redis_command_stats,
                                    Stats::Scope& scope, const std::string& auth_username,
                                    const std::string& auth_password) {
  ClientPtr client = ClientImpl::create(host, dispatcher, EncoderPtr{new EncoderImpl()},
                                        decoder_factory_, config, redis_command_stats, scope);
  client->initialize(auth_username, auth_password);
  return client;
}

ClientPtr ClientImpl::create(Upstream::HostConstSharedPtr host, Event::Dispatcher& dispatcher,
                             EncoderPtr&& encoder, DecoderFactory& decoder_factory,
                             const Config& config,
                             const RedisCommandStatsSharedPtr& redis_command_stats,
                             Stats::Scope& scope) {
  auto client = std::make_unique<ClientImpl>(host, dispatcher, std::move(encoder),
      decoder_factory, config, redis_command_stats, scope);
  // 注意这里创建了一个connection_
  // 有兴趣的同学可以看看这个过程是什么样的
  client->connection_ = host->createConnection(dispatcher, nullptr, nullptr).connection_;
  client->connection_->addConnectionCallbacks(*client);
  // 注意这里！！！！！
  // 注意这里！！！！！
  // 注意这里！！！！！
  client->connection_->addReadFilter(Network::ReadFilterSharedPtr{new UpstreamReadFilter(*client)});
  client->connection_->connect();
  client->connection_->noDelay(true);
  return client;
}
```

知道了`client`的来源之后，我们回到主流程上：

```c++
// /envoy/source/extensions/filters/network/common/redis/client_impl.cc
PoolRequest* ClientImpl::makeRequest(const RespValue& request, ClientCallbacks& callbacks) {
  ASSERT(connection_->state() == Network::Connection::State::Open);

  const bool empty_buffer = encoder_buffer_.length() == 0;

  Stats::StatName command;
  if (config_.enableCommandStats()) {
    // Only lowercase command and get StatName if we enable command stats
    command = redis_command_stats_->getCommandFromRequest(request);
    redis_command_stats_->updateStatsTotal(scope_, command);
  } else {
    // If disabled, we use a placeholder stat name "unused" that is not used
    command = redis_command_stats_->getUnusedStatName();
  }

  pending_requests_.emplace_back(*this, callbacks, command);
  // 将 request 序列化成字节
  // 序列化的过程也比较简单
  // 参见 /envoy/source/extensions/filters/network/common/redis/codec_impl.cc
  // 假设下游发来一个 get a请求，经过decoder变成 
  // RespValue{
  //     type_(RespType::Array)
  // 	   array_(RespValue{type_: RespType::BulkString, string_:'get'}, RespValue{type_: RespType::BulkString, string_:'a'})
	// }
  // encode完之后，又会变成 buffer_('$2\r\n*3\r\nget\r\n*1\r\na\r\n')
  encoder_->encode(request, encoder_buffer_);

  // If buffer is full, flush. If the buffer was empty before the request, start the timer.
  if (encoder_buffer_.length() >= config_.maxBufferSizeBeforeFlush()) {
    // 实际发送数据的地方
    flushBufferAndResetTimer();
  } else if (empty_buffer) {
    flush_timer_->enableTimer(std::chrono::milliseconds(config_.bufferFlushTimeoutInMs()));
  }

  // Only boost the op timeout if:
  // - We are not already connected. Otherwise, we are governed by the connect timeout and the timer
  //   will be reset when/if connection occurs. This allows a relatively long connection spin up
  //   time for example if TLS is being used.
  // - This is the first request on the pipeline. Otherwise the timeout would effectively start on
  //   the last operation.
  if (connected_ && pending_requests_.size() == 1) {
    connect_or_op_timer_->enableTimer(config_.opTimeout());
  }

  return &pending_requests_.back();
}
```

最后我们看看向上游发送数据的过程

```c++
// /envoy/source/extensions/filters/network/common/redis/client_impl.cc
void ClientImpl::flushBufferAndResetTimer() {
  if (flush_timer_->enabled()) {
    flush_timer_->disableTimer();
  }
  connection_->write(encoder_buffer_, false);
}
```

到这我们已经把从接收下游流量开始，经过解析，负载均衡，然后找到上游合适的机器，建立连接，发送数据完整的说完了，不知道大家是什么样的感受。当然里面还是留了一些坑，需要大家自己去探索，思考。

最后用一张完整的图来总结![上游到上游](https://github.com/x-lambda/note/blob/master/envoy/3/envoy-redis%E4%B8%8B%E6%B8%B8%E5%88%B0%E4%B8%8A%E6%B8%B8%E8%BF%87%E7%A8%8B.png)



本文的最后我想留一些问题给大家思考：

* 从最开始的`ProxyFilter::onRespValue()`到最后的`makeRequest()`我们一路传递了很多`request/callback`，这些形参实参始终都是一个对象吗？
* 从以前说过的`tcp filter`，我们知道一个完整的代理请求要有2个回调`onData()`和`onWrite()`，`onData()`我们已经看过了，那么`redis filter`这里的`onWrite()`在哪呢？

