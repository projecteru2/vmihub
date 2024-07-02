VMIhub
====
![](https://github.com/projecteru2/vmihub/workflows/test/badge.svg)
![](https://github.com/projecteru2/vmihub/workflows/golangci-lint/badge.svg)

virtual machine image hub for ERU

### Swagger
generate swagger
```shell
make swag
```

#### Install swagger on MacOS
https://github.com/swaggo/swag/blob/v1.16.1/README_zh-CN.md

Generate swagger
```shell
swag init -g cmd/vmihub/main.go -o cmd/vmihub/docs
```

### Redis
Require Redis

### DB
[ReadME](internal/models/migration/README.md)

### S3
Local can use minio

### Build 
```shell
make
```

### Start
```shell
bin/vmihub --config=config/config.example.toml server  
```
