vmihub
====
![](https://github.com/projecteru2/vmihub/workflows/test/badge.svg)
![](https://github.com/projecteru2/vmihub/workflows/golangci-lint/badge.svg)

virtual machine image hub for ERU

# swagger
generate swagger
```shell
make swag
```

### Mac 下安装swagger
https://github.com/swaggo/swag/blob/v1.16.1/README_zh-CN.md

generate swagger
```shell
swag init -g cmd/vmihub/main.go -o cmd/vmihub/docs
```
# 准备redis
常规准备就好

# 准备数据库
不管是第一次初始化数据库还是后续数据库schema的修改请都参考 [这里](internal/models/migration/README.md)
# 准备s3
用于存储镜像文件

# 编译

```shell
make
```

# 项目启动
```shell
bin/vmihub --config=config/config.example.toml server  
```
