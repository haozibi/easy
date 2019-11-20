# singleflight

当多个完全相同请求同时访问时，只会有一个请求“真正”执行方法，其他请求返回相同结果

## 性能

- 未使用 singleflight

```console
$ wrk -t100 -c100 -d30 -T30s -H "X-Fly: abc" --latency http://127.0.0.1:9091/original
Running 30s test @ http://127.0.0.1:9091/original
  100 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    22.09ms    1.77ms  35.51ms   82.77%
    Req/Sec    45.20      5.18    58.00     57.97%
  Latency Distribution
     50%   21.50ms
     75%   22.79ms
     90%   24.48ms
     99%   28.37ms
  135954 requests in 30.11s, 23.56MB read
Requests/sec:   4515.89
Transfer/sec:    801.31KB

$ curl 127.0.0.1:9091/count
136043
```

- 使用 singleflight

```console
$ wrk -t100 -c100 -d30 -T30s -H "X-Fly: abc" --latency http://127.0.0.1:9091/single
Running 30s test @ http://127.0.0.1:9091/single
  100 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    24.05ms    2.30ms  34.55ms   68.62%
    Req/Sec    41.47      4.45    50.00     77.04%
  Latency Distribution
     50%   23.84ms
     75%   25.55ms
     90%   27.18ms
     99%   29.83ms
  124874 requests in 30.08s, 21.65MB read
Requests/sec:   4151.59
Transfer/sec:    737.21KB

$ curl 127.0.0.1:9091/count
1251
```