# `Envoy-Redis`源码分析 第5章

### 序

前一篇文章我们已经定位到了`handler`行为，但苦于出现一个未知的对象`router`，我们并没有继续跟踪代码，只是猜测了后续行为。那么灵魂三问来了，这个玩意是啥？从哪来？又要到哪里去？



要解释这个问题，我们还得从最初的开始说起。`envoy`把上游的一个服务对应的实例信息划为一个`cluster`，看看官方的定义

```shell
Cluster: A cluster is a group of logically similar upstream hosts that Envoy connects to. Envoy discovers the members of a cluster via service discovery. It optionally determines the health of cluster members via active health checking. The cluster member that Envoy routes a request to is determined by the load balancing policy.
```

说白了，`envoy`把上游的机器信息存储到一个`cluster`对象中。这个`cluster`可以是静态的，例如我们在配置中写死实例，常见的`Redis/MySQL/Memcached`集群，这些实例`IP`基本是固定的，直接写到配置文件里比较方便。还有一类是动态的，通过服务发现拿到的，例如我们的服务依赖上游的支付服务，每次他们发版的时候，实例的`IP`就会发生变化，此时通过服务发现拿到变更后的实例信息更方便。

我们先看这两种对应的配置长啥样

##### `static cluster config`

```yaml
clusters:
	- name: i_am_redis_cluster
      cluster_type:
        name: envoy.clusters.redis
        typed_config:
          "@type": type.googleapis.com/google.protobuf.Struct
          value:
            cluster_refresh_rate: 60s
      lb_policy: CLUSTER_PROVIDED
      connect_timeout: 0.25s
      load_assignment:
        cluster_name: i_am_redis_cluster
        endpoints:
          - lb_endpoints:
            - endpoint:
                address:
                  socket_address: { address: 10.22.33.44, port_value: 6839 }
            - endpoint:
                address:
                  socket_address: { address: 10.22.33.44, port_value: 6841 }
            - endpoint:
                address:
                  socket_address: { address: 10.22.33.44, port_value: 6839 }

```

可以看见这里上游的`Redis`集群中有3个实例，对应的`IP`和端口都是固定的

##### `dynamic cluster config`

```yaml
clusters:
  - name: internal.service
    connect_timeout: 0.25s
    lb_policy: ROUND_ROBIN
    type: EDS
    eds_cluster_config:
      service_name: api.internal
      eds_config:
        api_config_source:
          api_type: REST
          cluster_names: [internal_cluster]
          refresh_delay: 5s
  - name: internal_cluster
    connect_timeout: 0.25s
    type: STATIC
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    load_assignment:
      cluster_name: internal_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: 127.0.0.1
                port_value: 14250
```

动态的配置复杂一点，我们想要得到上游集群的实例信息，需要去服务中心获取，这里服务中心的我随便给了一个`IP`和端口。然后`envoy`会发起指定的`HTTP/GRPC`请求，服务中心那边按指定的pb格式返回实例信息即可。



`envoy`支持不同类型的上游`cluster`，例如：

* `static`
* `eds`
* `strict_dns`
* `redis`

所有支持的`cluster`可以参考`/envoy/source/common/upstream`下的文件。`envoy`配置文件能化为2部分理解，1是`listener`配置，另1个是`cluster`配置。`listener`会关联一个`cluster`。

还记得我们前面文章中的`tcp filter`例子吗？我们当时只是收到数据之后，打印一条日志，然后就返回了。我们并没有往上游转发，如果此时我们上游`cluster`中有多个实例，而我们想要转给其中一个，那我们需要怎么做呢？

代理通常就是干这事的，`envoy`也是代理，所以`envoy`当然支持。如果下游请求转发给上游的任意一台机器结果都一样，即这个请求是无状态的，那么我们可以简单点做，在`cluster`配置中，指定一个配置。

```yaml
clusters:
    - name: i_am_cluster
      type: strict_dns
      lb_policy: round_robin  # 这个配置很重要，我们以后会反复用到
```

这样`envoy`就会帮我们管理和上游`cluster`之间的连接，下游一个请求过来，`envoy`会根据某种策略找到对应的机器，然后将请求转发过去。这样从`listener`到`cluster`就串起来了。当然有时候`envoy`为我们准备好的并不是我们想要的，这时候我们也可以定制自己想要的`cluster`，这也意味着连接池，健康检查，负载均衡都需要我们自己来实现。

那么`envoy`是怎么支持这些`cluster`的呢？

首先`cluster`也是注册制的，我们可以看看代码

##### `static cluster`

```c++
// 注册
REGISTER_FACTORY(StaticClusterFactory, ClusterFactory);

/**
 * Factory for StaticClusterImpl cluster.
 */
class StaticClusterFactory : public ClusterFactoryImplBase {
public:
  // 继承自 ClusterFactoryImplBase 对象
  // 构造函数会传递一个 name， name的定义见下面
  StaticClusterFactory()
      : ClusterFactoryImplBase(Extensions::Clusters::ClusterTypes::get().Static) {}
  
  // StaticClusterFactory 同时会实现一个方法 createClusterImpl() 
  // 这点很重要
};

class ClusterTypeValues {
public:
  // Static clusters (cluster that have a fixed number of hosts with resolved IP addresses).
  const std::string Static = "envoy.cluster.static";
}
```

##### `logical_dns cluster`

```c++
// 注册 
REGISTER_FACTORY(LogicalDnsClusterFactory, ClusterFactory);

class LogicalDnsClusterFactory : public ClusterFactoryImplBase {
public:
  // 继承自 ClusterFactoryImplBase 对象
  // 构造函数会传递一个 name， name的定义见下面
  LogicalDnsClusterFactory()
      : ClusterFactoryImplBase(Extensions::Clusters::ClusterTypes::get().LogicalDns) {}
  
  // LogicalDnsClusterFactory 同时会实现一个方法 createClusterImpl() 
  // 这点很重要
};

class ClusterTypeValues {
public:
  // Logical DNS (cluster that creates a single logical host that wraps an async DNS resolver).
  const std::string LogicalDns = "envoy.cluster.logical_dns";
}
```

##### `original_dst cluster`

```c++
/**
 * Static registration for the original dst cluster factory. @see RegisterFactory.
 */
REGISTER_FACTORY(OriginalDstClusterFactory, ClusterFactory);

class OriginalDstClusterFactory : public ClusterFactoryImplBase {
public:
  // 继承自 ClusterFactoryImplBase 对象
  // 构造函数会传递一个 name， name的定义见下面
  OriginalDstClusterFactory()
      : ClusterFactoryImplBase(Extensions::Clusters::ClusterTypes::get().OriginalDst) {}

  // OriginalDstClusterFactory 同时会实现一个方法 createClusterImpl() 
  // 这点很重要
};

class ClusterTypeValues {
public:
  // Original destination (dynamic cluster that automatically adds hosts as needed based on the
  // original destination address of the downstream connection).
  const std::string OriginalDst = "envoy.cluster.original_dst";
}
```

##### `eds cluster`

```c++
/**
 * Static registration for the Eds cluster factory. @see RegisterFactory.
 */
REGISTER_FACTORY(EdsClusterFactory, ClusterFactory);

class EdsClusterFactory : public ClusterFactoryImplBase {
public:
  // 继承自 ClusterFactoryImplBase 对象
  // 构造函数会传递一个 name， name的定义见下面
  EdsClusterFactory() : ClusterFactoryImplBase(Extensions::Clusters::ClusterTypes::get().Eds) {}
  // EdsClusterFactory 同时会实现一个方法 createClusterImpl() 
  // 这点很重要
};

class ClusterTypeValues {
public:
  // Endpoint Discovery Service (dynamic cluster that reads host information from the Endpoint
  // Discovery Service).
  const std::string Eds = "envoy.cluster.eds";
}
```

##### `redis cluster`

```c++
REGISTER_FACTORY(RedisClusterFactory, Upstream::ClusterFactory);

class RedisClusterFactory : public Upstream::ConfigurableClusterFactoryBase<
                                envoy::config::cluster::redis::RedisClusterConfig> {
public:
  // RedisClusterFactory 有点特殊，继承的是 ConfigurableClusterFactoryBase
  RedisClusterFactory()
      : ConfigurableClusterFactoryBase(Extensions::Clusters::ClusterTypes::get().Redis) {}
	
  // 实现了 createClusterWithConfig 方法
};

class ClusterTypeValues {
public:
  // Redis cluster (cluster that reads host information using the redis cluster protocol).
  const std::string Redis = "envoy.clusters.redis";
}
```

看完上面那么多的`cluster`代码，大概读者也猜到了，如果再想要扩展`envoy cluster`，差不多也是同样的流程。选择一个不会重复的`name`，然后继承某个对象，再实现一个指定的方法。这样做的意图先不表，后面我再慢慢道来。



##### 启动过程

`envoy`在启动时，会先加载解析配置，然后初始化`Admin API`，`worker`等等。然后才开始创建`cluster`。

启动时的过程比较繁琐，涉及配置加载，`worker`线程等，这里我们直奔主题，其他流程可以参考以下文章

* [Envoy源码分析之一--Server初始化（上）](https://bbs.huaweicloud.com/blogs/151195)

* [Envoy源码分析之一--Server初始化（下）](https://bbs.huaweicloud.com/blogs/151199)



我们从`main()`函数一路找下去，可以找到

```c++
// /envoy/source/exe/main_common.cc
server_ = std::make_unique<Server::InstanceImpl>(
        *init_manager_, options_, time_system, local_address, listener_hooks, *restarter_,
        *stats_store_, access_log_lock, component_factory, std::move(random_generator), 
        *tls_, thread_factory_, file_system_, std::move(process_context));

// /envoy/source/server/server.cc
void InstanceImpl::initialize(const Options& options,
                              Network::Address::InstanceConstSharedPtr local_address,
                              ComponentFactory& component_factory, ListenerHooks& hooks) {
  
  // 创建ProdClusterManagerFactory对象
  cluster_manager_factory_ = std::make_unique<Upstream::ProdClusterManagerFactory>(
      *admin_, Runtime::LoaderSingleton::get(), stats_store_, thread_local_, dns_resolver_,
      *ssl_context_manager_, *dispatcher_, *local_info_, *secret_manager_,
      messageValidationContext(), *api_, http_context_, grpc_context_, router_context_,
      access_log_manager_, *singleton_manager_);
  
  // MainImpl::initialize()
  config_.initialize(bootstrap_, *this, *cluster_manager_factory_);
}

// /envoy/source/server/configuration_impl.cc
void MainImpl::initialize(const envoy::config::bootstrap::v3::Bootstrap& bootstrap,
                          Instance& server,
                          Upstream::ClusterManagerFactory& cluster_manager_factory) {
  // cluster_manager_ 实际是 ClusterManagerImpl 对象
  // clusterManagerFromProto() 只new了这个对象
  cluster_manager_ = cluster_manager_factory.clusterManagerFromProto(bootstrap);
}

// /envoy/source/common/upstream/cluster_manager_impl.cc
ClusterManagerPtr ProdClusterManagerFactory::clusterManagerFromProto(
    const envoy::config::bootstrap::v3::Bootstrap& bootstrap) {
  // 注意这里的实参传了 *this
  // 在形参中这个参数叫 factory
  return ClusterManagerPtr{new ClusterManagerImpl(
      bootstrap, *this, stats_, tls_, runtime_, local_info_, log_manager_, 		
    	main_thread_dispatcher_, admin_, validation_context_, api_, 
    	http_context_, grpc_context_, router_context_)};
}

// /envoy/source/common/upstream/cluster_manager_impl.cc
ClusterManagerImpl::ClusterManagerImpl(
    const envoy::config::bootstrap::v3::Bootstrap& bootstrap, 
  	ClusterManagerFactory& factory, Stats::Store& stats, 
  	ThreadLocal::Instance& tls, Runtime::Loader& runtime,
    const LocalInfo::LocalInfo& local_info, AccessLog::AccessLogManager& log_manager,
    Event::Dispatcher& main_thread_dispatcher, Server::Admin& admin,
    ProtobufMessage::ValidationContext& validation_context, Api::Api& api,
    Http::Context& http_context, Grpc::Context& grpc_context, 
  	Router::Context& router_context) {
  
    // Cluster loading happens in two phases: first all the primary clusters are loaded, and then all
    // the secondary clusters are loaded. As it currently stands all non-EDS clusters and EDS which
    // load endpoint definition from file are primary and
    // (REST,GRPC,DELTA_GRPC) EDS clusters are secondary. This two phase
    // loading is done because in v2 configuration each EDS cluster individually sets up a
    // subscription. When this subscription is an API source the cluster will depend on a non-EDS
    // cluster, so the non-EDS clusters must be loaded first.
    auto is_primary_cluster = [](const envoy::config::cluster::v3::Cluster& cluster) -> bool {
    return cluster.type() != envoy::config::cluster::v3::Cluster::EDS ||
           (cluster.type() == envoy::config::cluster::v3::Cluster::EDS &&
            cluster.eds_cluster_config().eds_config().config_source_specifier_case() ==
                envoy::config::core::v3::ConfigSource::ConfigSourceSpecifierCase::kPath);
  };
    // envoy 在这里把 cluster 分成两种
  	// primary cluster
  	// secondary cluster
    //这里也很好理解，`cluster`类型不是`EDS`的就算`primary cluster`，如果是`EDS`，则要满足特定的条件，也可以算`primary cluster`。这两种`cluster`主要的区别就在于初始化的阶段。
  
  // Load all the primary clusters. 加载 primary cluster
  for (const auto& cluster : bootstrap.static_resources().clusters()) {
    if (is_primary_cluster(cluster)) {
      // 我们接着看这个调用做了什么
      loadCluster(cluster, "", false, active_clusters_);
    }
  }
}
```

经过一番跟踪，终于找到`envoy`加载`cluster`的代码，我们再接着看

```c++
// /envoy/source/common/upstream/cluster_manager_impl.cc
ClusterManagerImpl::ClusterDataPtr
ClusterManagerImpl::loadCluster(const envoy::config::cluster::v3::Cluster& cluster,
                                const std::string& version_info, bool added_via_api,
                                ClusterMap& cluster_map) {
  // 核心实现在这
  // 这里的 factory_ 就是在 new ClusterManagerImpl() 时传的 this
  // 也就是 ProdClusterManagerFactory 对象
  std::pair<ClusterSharedPtr, ThreadAwareLoadBalancerPtr> new_cluster_pair =
      factory_.clusterFromProto(cluster, *this, outlier_event_logger_, added_via_api);
  auto& new_cluster = new_cluster_pair.first;
  Cluster& cluster_reference = *new_cluster;

  // ....
}

// /envoy/source/common/upstream/cluster_manager_impl.cc
// 继续跟踪
std::pair<ClusterSharedPtr, ThreadAwareLoadBalancerPtr> ProdClusterManagerFactory::clusterFromProto(
    const envoy::config::cluster::v3::Cluster& cluster, ClusterManager& cm,
    Outlier::EventLoggerSharedPtr outlier_event_logger, bool added_via_api) {
  // 注意 ClusterFactoryImplBase 有2个 create() 方法
  // 这里调用的是参数多的那个函数
  return ClusterFactoryImplBase::create(
      cluster, cm, stats_, tls_, dns_resolver_, ssl_context_manager_, runtime_,
      main_thread_dispatcher_, log_manager_, local_info_, admin_, singleton_manager_,
      outlier_event_logger, added_via_api,
      added_via_api ? validation_context_.dynamicValidationVisitor()
                    : validation_context_.staticValidationVisitor(),
      api_);
}

// /envoy/source/common/upstream/cluster_manager_impl.cc
std::pair<ClusterSharedPtr, ThreadAwareLoadBalancerPtr> ClusterFactoryImplBase::create(
    const envoy::config::cluster::v3::Cluster& cluster, ClusterManager& cluster_manager,
    Stats::Store& stats, ThreadLocal::Instance& tls, Network::DnsResolverSharedPtr dns_resolver,
    Ssl::ContextManager& ssl_context_manager, Runtime::Loader& runtime,
    Event::Dispatcher& dispatcher, AccessLog::AccessLogManager& log_manager,
    const LocalInfo::LocalInfo& local_info, Server::Admin& admin,
    Singleton::Manager& singleton_manager, Outlier::EventLoggerSharedPtr outlier_event_logger,
    bool added_via_api, ProtobufMessage::ValidationVisitor& validation_visitor, Api::Api& api) {
  std::string cluster_type;

  // 判断 cluster 类型
  if (!cluster.has_cluster_type()) {
    switch (cluster.type()) {
    case envoy::config::cluster::v3::Cluster::STATIC:
      cluster_type = Extensions::Clusters::ClusterTypes::get().Static;
      break;
    case envoy::config::cluster::v3::Cluster::STRICT_DNS:
      cluster_type = Extensions::Clusters::ClusterTypes::get().StrictDns;
      break;
    case envoy::config::cluster::v3::Cluster::LOGICAL_DNS:
      cluster_type = Extensions::Clusters::ClusterTypes::get().LogicalDns;
      break;
    case envoy::config::cluster::v3::Cluster::ORIGINAL_DST:
      cluster_type = Extensions::Clusters::ClusterTypes::get().OriginalDst;
      break;
    case envoy::config::cluster::v3::Cluster::EDS:
      cluster_type = Extensions::Clusters::ClusterTypes::get().Eds;
      break;
    default:
      NOT_REACHED_GCOVR_EXCL_LINE;
    }
  } else {
    cluster_type = cluster.cluster_type().name();
  }

  // ....
  
  // 这里根据cluster type查找对应的工厂对象
  // 还记得文章开始的时候，我们说过cluster注册，每个cluster有一个工厂类型，然后注册到一个map中
  // 这里就是根据name去map中反查cluster factory
  ClusterFactory* factory = Registry::FactoryRegistry<ClusterFactory>::getFactory(cluster_type);

  if (factory == nullptr) {
    throw EnvoyException(fmt::format(
        "Didn't find a registered cluster factory implementation for name: '{}'", cluster_type));
  }

  ClusterFactoryContextImpl context(
      cluster_manager, stats, tls, std::move(dns_resolver), ssl_context_manager, runtime,
      dispatcher, log_manager, local_info, admin, singleton_manager,
      std::move(outlier_event_logger), added_via_api, validation_visitor, api);
  
  // cluster factory继承自 ClusterFactoryImplBase但没复写create()方法
  // 所以这里等同于调用 ClusterFactoryImplBase::create() 两个参数的那个方法
  return factory->create(cluster, context);
}

// /envoy/source/common/upstream/cluster_factory_impl.cc
std::pair<ClusterSharedPtr, ThreadAwareLoadBalancerPtr>
ClusterFactoryImplBase::create(const envoy::config::cluster::v3::Cluster& cluster,
                               ClusterFactoryContext& context) {
  auto stats_scope = generateStatsScope(cluster, context.stats());
  Server::Configuration::TransportSocketFactoryContextImpl factory_context(
      context.admin(), context.sslContextManager(), *stats_scope, context.clusterManager(),
      context.localInfo(), context.dispatcher(), context.stats(), context.singletonManager(),
      context.tls(), context.messageValidationVisitor(), context.api());
	
  // 调用 createClusterImpl()
  // 还记得我们前面说过的 cluster 注册的时候
  // cluster factory都实现了一个 createClusterImpl() 方法。
  // 所以这里相当于调用
  //				* StaticClusterFactory.createClusterImpl()
  //				* LogicalDnsClusterFactory.createClusterImpl()
  //				* OriginalDstClusterFactory.createClusterImpl()
  //				* EdsClusterFactory.createClusterImpl()
  std::pair<ClusterImplBaseSharedPtr, ThreadAwareLoadBalancerPtr> new_cluster_pair =
      createClusterImpl(cluster, context, factory_context, std::move(stats_scope));

	// ...
  return new_cluster_pair;
}
```

又是层层的套娃，不过最终我们还是找到了加载`cluster`的过程。之间曲折婉转，柳暗花明，尤其是`ClusterManagerFactory`/`ClusterManager`/`ClusterFactory`这些对象特别容易混淆，还需要读者自己不断的去阅读，理解。



参考：

* [Envoy 的架构与基本配置解析](https://jimmysong.io/blog/envoy-archiecture-and-terminology/)
* [Envoy cluster api](https://www.envoyproxy.io/docs/envoy/latest/api-v3/clusters/clusters)

