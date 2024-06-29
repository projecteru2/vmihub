## 安装migrate 
直接取 [这里](https://github.com/golang-migrate/migrate/releases)下载，或者运行`make db-migrate-setup`

## 创建up和down文件

```
make db-migrate-create table=xxx
```
或者直接跑migrate命令
```
migrate create -ext sql -dir internal/models/migration op_log_table
```

## 手动编写up和down文件，up是应用文件，down是回滚文件
```
参考：
20231227072912_op_log_table.up.sql
20231227072912_op_log_table.down.sql
```

## 迁移

```
make db-migrate-up uri='mysql://vmihub:******@tcp(10.200.0.188:3306)/vmihub_test?parseTime=true'
```

或者直接运行migrate命令
```
migrate -database 'mysql://vmihub:******@tcp(10.200.0.188:3306)/vmihub_test?parseTime=true' -path ./internal/models/migration up 1
```

## 回滚
```
make db-migrate-down uri='mysql://vmihub:******@tcp(10.200.0.188:3306)/vmihub_test?parseTime=true' N=1
```

或者直接运行migrate命令
```
migrate -database 'mysql://vmihub:******@tcp(10.200.0.188:3306)/vmihub_test?parseTime=true' -path ./internal/models/migration down
```