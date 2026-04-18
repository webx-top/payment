项目：payment（中文文档）
======================

简介
----

`payment` 是一个通用的 Go 语言支付库，提供统一的驱动接口以支持多种支付平台（例如支付宝、微信、PayPal、PayJS、MugglePay、XunhuPay 等）。

主要特性
----
- 统一的 `Driver` 接口，便于新增与切换不同支付实现。
- 提供账户配置、支付/查询/退款/回调等常用功能抽象。
- 内置对常见支付渠道的驱动（位于 `driver/` 目录）。

安装
----

在你的模块中使用：

```bash
go get github.com/webx-top/payment
```

快速开始
----

1. 在项目中引入并注册驱动（驱动通常在 `driver` 包内自注册）：

```go
import (
    "github.com/webx-top/payment"
)

// 获取驱动工厂并创建实例
drvFactory := payment.Get("alipay")
if drvFactory != nil {
    drv := drvFactory()
    // 设置账户、回调等
}
```

核心概念
----

- `Driver`：驱动接口，定义了支付（Pay）、查询（PayQuery/RefundQuery）、退款（Refund）、回调处理（PayNotify/RefundNotify）以及签名验证（VerifySign）等方法。实现该接口即可加入平台支持。
- `config` 包：包含 `Account`、`Pay`、`Query`、`Refund`、`Result` 等结构体，用于传递配置与请求/响应数据。

主要导出函数与用法
----

- `payment.Register(platform string, name string, hook func() Driver, setDefaults ...func(*config.Account))`：注册平台驱动与默认账户设置函数。
- `payment.Get(platform string) func() Driver`：获取对应平台的驱动工厂函数。
- `payment.Platforms()`：返回已注册的平台映射（platform => 名称）。
- `payment.Name(platform string)`：返回平台中文名或显示名。

配置说明（概览）
----

配置项位于 `config` 包，常见结构体：

- `Account`：平台账户信息（如 appid、密钥、商户号等），不同驱动会使用不同字段。
- `Pay`：发起支付所需参数（金额、描述、回调地址等）。
- `Query`：用于查询订单或退款。
- `Refund`：退款请求参数。
- `Result`：支付/退款/查询返回的标准结构。

驱动列表（目录）
----

本仓库在 `driver/` 下包含多个驱动目录：

- `alipay`：支付宝相关实现。
- `wechat`：微信支付实现。
- `paypal`：PayPal 支付实现。
- `payjs`：PayJS 支持。
- `mugglepay`：MugglePay（加密货币）实现。
- `epusdt`：EPUSDT（USDT 相关）实现。
- `mockpay`：测试/模拟支付网关，用于开发与测试。
- `xunhupay`：迅虎/虎皮椒类支付实现。

（每个子目录内一般包含各自的 `helpers.go`、实现文件和测试用例，查看子目录以获取具体用法与示例）

回调与安全提示
----

- 驱动实现必须在 `PayNotify` 与 `RefundNotify` 中自行验证签名或请求合法性，确保安全。
- 对外暴露的回调处理函数应尽量快速返回，异步处理业务逻辑。

示例：注册并发起支付（伪代码）
----

```go
package main

import (
    "github.com/webx-top/payment"
    "github.com/webx-top/payment/config"
)

func main() {
    // 假设驱动已通过 init() 自动注册
    factory := payment.Get("alipay")
    if factory == nil {
        panic("alipay driver not registered")
    }
    drv := factory()
    acc := &config.Account{ /* 填写账号信息 */ }
    drv.SetAccount(acc)
    // 调用 Pay 等方法完成支付流程
}
```
