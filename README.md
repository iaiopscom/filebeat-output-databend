# filebeat-output-databend

本项目为filebeat输出插件，支持将事件推送到clickhouse数据表，您需要在filebeat中引入本项目并且重新编译filebeat。

## 一、安装编译

### 1、下载beats

```shell
git clone git@github.com:elastic/beats.git
```

### 2、更改beats outputs includes文件, 添加filebeat-output-databend

includes所在目录：beats/libbeat/publisher/includes/includes.go  
添加如下代码：

```go
import (
...
_ "gitee.com/superqy/filebeat-output-databend"
)
```

### 3、构建filebeat

filebeat所在目录：beats/filebeat  
执行如下命令：

```shell
make
```

## 二、配置

### filebeat-out-databend插件配置

```yml
#================================ Databend output ======================
#----------------------------- Databend output --------------------------------
output.databend:
  # databend数据库配置
  # https://github.com/databendcloud/databend-go
  # 示例 https://{USER}:{PASSWORD}@{WAREHOUSE_HOST}:443/{DATABASE}
  url: "http://127.0.0.1:9000/database"
  # 接收数据的表名
  table: ck_test_v1
  # 数据过滤器的表列，匹配日志文件中对应的键
  columns: ["id", "name", "created_date"]
  # 异常重试休眠时间 单位：秒
  retry_interval: 3
  # 是否跳过异常事件推送 true-表示跳过执行异常实践 false-会一直重试，重试间隔为retry_interval
  skip_unexpected_type_row: true
```
