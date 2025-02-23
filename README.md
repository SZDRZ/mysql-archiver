# mysql-archiver

Languages: [English](README.en.md)

## 介绍
这是一款适用于MySQL跨服务器归档场景的工具。能够实现把A实例的数据转移到B实例上。目前支持单表归档、多表联级归档，满足大多数归档需求。
使用时您只需通过YAML定义好归档参数（如批次大小、归档间隔、事务隔离级别等）、归档规则即可。剩下的循环归档动作交由`mysql-archiver`去执行~

### !!! For release DBA hands. !!!

![](docs/image1.png)

![](docs/image2.png)


## 使用说明

1. 请准备好您的Docker环境或者Golang环境（要求>=1.21.10）
2. 运行编译脚本`build.sh`
3. 根据自己的需求修改YAML配置文件（[查看配置文件说明](#配置文件说明)）
4. 运行程序


# 配置文件说明


```yaml
global:
  batch_size: 100      # 归档批次大小; 即每张表按照条件抽取多少行数据
  sleep: 1s          # 批次间隔
  datasource:         # 数据源的配置
    transaction_isolation: REPEATABLE READ     # 源库事务隔离级别
    src:       # 源库 账号密码库名...
      addr: 127.0.0.1:3306
      user: root
      pass: "1234"
      dbname: test
    dst:       # 归档目标库
      addr: 192.168.66.3:3306
      user: root
      pass: "123456"
      dbname: aaa
rules:         # 归档规则配置（每条规则按配置顺序从上往下执行，直到当前表数据全部归档完毕才会开始进行下一张表的归档）
  # 单表归档示例
  - table: ApplicationLogs
    where: CreationTime < DATE_SUB(DATE(NOW()), INTERVAL 6 MONTH)
    pk: Id

  # 多表联级归档示例
  - table: Orders         # 主表名
    where: 1=1            # 归档条件
    pk: Id                # 主键名
    deps:                 # 子表依赖项
      - table: OrderDetails       # 子表名
        pk: DetailId              # 子表主键名
        key: OrderId              # 与主表主键关联的键名
  - table: products
    where: 1 = 1
    pk: product_id
    deps:
      - table: product_packages
        pk: package_id
        key: product_id
        deps:
          - table: package_barcodes
            pk: barcode_id
            key: package_id
```
