项目：payment（中文文档）
======================

简介
----

`payment` 是一个通用的 Go 语言支付库，提供统一的驱动接口以支持多种支付平台（例如支付宝、微信、PayPal、Authorize.Net、Mollie、Razorpay、银联、Stripe 等）。

主要特性
----
- 统一的 `Driver` 接口，便于新增与切换不同支付实现。
- 提供账户配置、支付/查询/退款/退款通知/回调等常用功能抽象。
- 内置对常见支付渠道的驱动（位于 `driver/` 目录）。
- 支持多种签名验证方式（HMAC-SHA256、RSA、MD5 Hash 等）。

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

### Driver 接口

驱动接口定义了以下核心方法：

| 方法 | 说明 |
|------|------|
| `Pay(ctx, cfg)` | 发起支付 |
| `PayNotify(ctx)` | 处理支付回调通知 |
| `PayQuery(ctx, cfg)` | 查询支付状态 |
| `Refund(ctx, cfg)` | 发起退款 |
| `RefundNotify(ctx)` | 处理退款回调通知 |
| `RefundQuery(ctx, cfg)` | 查询退款状态 |
| `VerifySign(ctx)` | 签名验证 |
| `SetAccount(account)` | 设置账户配置 |
| `IsSupported(support)` | 检查是否支持某功能 |

### config 包核心结构体

| 结构体 | 说明 |
|--------|------|
| `Account` | 平台账户信息（AppID、AppSecret、MerchantID、PublicKey、PrivateKey 等） |
| `Pay` | 支付请求参数（Amount、Currency、Subject、OutTradeNo、NotifyURL、ReturnURL 等） |
| `Query` | 查询参数（TradeNo / RefundNo / OutTradeNo / OutRefundNo） |
| `Refund` | 退款请求参数（TradeNo、OutTradeNo、OutRefundNo、RefundAmount 等） |
| `Result` | 统一响应结果（Operation、Status、TradeNo、OutTradeNo、Currency、TotalAmount、RefundFee、RefundNo、OutRefundNo、Raw 等） |

### Operation 与 Status 常量

- **Operation**：`OperationPayment`（支付）、`OperationRefund`（退款），区分当前操作类型。
- **Status**：`TradeStatusSuccess`（成功）、`TradeStatusClosed`（关闭/失败）、`TradeStatusProcessing`（处理中）、`TradeStatusWaitBuyerPay`（待支付）、`TradeStatusException`（异常）、`TradeStatusFinished`（已完成）。

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
  - `AppID`：应用 ID / API Key / 登录 ID（各平台命名不同）
  - `AppSecret`：应用密钥 / API Secret / 交易密钥
  - `MerchantID`：商户号
  - `PublicKey`：公钥（用于 RSA 签名场景）
  - `PrivateKey`：私钥
  - `Debug`：是否使用沙箱/测试环境
  - `Options.Extra`：扩展配置（如 apiURL、clientKey、md5Key 等）
- `Pay`：发起支付所需参数（金额、描述、回调地址等）。
- `Query`：用于查询订单或退款。
- `Refund`：退款请求参数。
- `Result`：支付/退款/查询返回的标准结构。

驱动列表（目录）
----

本仓库在 `driver/` 下包含多个驱动目录：

### 国内支付

| 驱动目录 | 名称 | 说明 | 支持功能 |
|----------|------|------|----------|
| `alipay` | 支付宝 | 支付宝支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery |
| `wechat` | 微信支付 | 微信支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery |
| `unionpay` | UnionPay | 银联支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery |

### 国际信用卡/在线支付

| 驱动目录 | 名称 | 说明 | 支持功能 | 支持币种 |
|----------|------|------|----------|----------|
| `authorize` | Authorize.Net | Authorize.Net 网关支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery | USD |
| `mollie` | Mollie | 欧洲 iDEAL/Bancontact 等多方式支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery | 多币种(EUR/USD/GBP...) |
| `razorpay` | Razorpay | 印度市场主流支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery | INR/USD/EUR/GBP... |
| `stripe` | Stripe | Stripe 支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery | - |
| `square` | Square | Square 支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery | - |
| `paddle` | Paddle | Paddle SaaS 订阅支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery | - |
| `twoco` | 2Checkout (TwoCo) | 2Checkout 支付网关 | Pay, PayNotify, PayQuery | - |
| `adyen` | Adyen | Adyen 全球支付平台 | - | - |

### 其他支付

| 驱动目录 | 名称 | 说明 | 支持功能 |
|----------|------|------|----------|
| `paypal` | PayPal | PayPal 支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify, RefundQuery |
| `payjs` | PayJS | PayJS 微信支付聚合 | Pay, PayNotify, PayQuery, Refund |
| `mugglepay` | MugglePay | 加密货币支付 | Pay, PayNotify, PayQuery, Refund, RefundNotify |
| `epusdt` | EPUSDT | USDT/TRC20 支付 | Pay, PayNotify, PayQuery |
| `mockpay` | MockPay | 测试/模拟支付网关 | Pay, PayNotify, PayQuery, Refund, RefundNotify |
| `xunhupay` | XunhuPay | 迅虎/虎皮椒支付 | Pay, PayNotify, PayQuery |

> 注：每个子目录内一般包含各自的 `helpers.go`（辅助函数/工具方法）、主实现文件，查看子目录源码以获取具体用法与示例。

回调与安全提示
----

- 驱动实现必须在 `PayNotify` 与 `RefundNotify` 中自行验证签名或请求合法性，确保安全。
  - **Authorize.Net**: MD5 Hash 验证 (`x_md5_hash`)
  - **Razorpay**: HMAC-SHA256 签名验证 (`X-Razorpay-Signature`)
  - **UnionPay**: RSA 公钥验签
  - **Mollie**: Bearer Token + IP 白名单
- **`PayNotify` 和 `RefundNotify` 必须独立实现**，不能复用同一逻辑：
  - `PayNotify` 的 `Operation` 为 `OperationPayment`
  - `RefundNotify` 的 `Operation` 为 `OperationRefund`
  - 两者使用的状态码映射也可能不同（如银联 `MappingRespCode` vs `MapRefundRespCode`）
- **`RefundNotify` 必须正确填充退款字段**：`RefundNo`（平台退款单号）、`RefundFee`（退款金额）、`OutRefundNo`（商户退款单号）
- 对外暴露的回调处理函数应尽量快速返回，异步处理业务逻辑。

示例：注册并发起支付（伪代码）
----

```go
package main

import (
    "github.com/webx-top/payment"
    "github.com/webx-top/payment/config"
    "github.com/webx-top/echo"
)

func main() {
    // 假设驱动已通过 init() 自动注册
    factory := payment.Get("alipay")
    if factory == nil {
        panic("alipay driver not registered")
    }
    drv := factory()

    // 设置账户
    acc := &config.Account{
        AppID:     "your_app_id",
        AppSecret: "your_app_secret",
        Debug:     true, // 沙箱模式
    }
    drv.SetAccount(acc)

    // 设置回调处理
    drv.SetNotifyCallback(func(ctx echo.Context, result *config.Result) error {
        if result.Operation == config.OperationPayment {
            // 处理支付结果
            println("支付:", result.Status, "金额:", result.TotalAmount)
        } else if result.Operation == config.OperationRefund {
            // 处理退款结果
            println("退款:", result.Status, "退款单号:", result.RefundNo)
        }
        return nil
    })

    // 发起支付
    payCfg := &config.Pay{
        Amount:      100.00,
        Currency:    config.NewCurrency("CNY"),
        Subject:     "测试订单",
        OutTradeNo:  "ORDER20260427001",
        NotifyURL:   "https://example.com/pay/notify",
        ReturnURL:   "https://example.com/pay/return",
    }
    resp, err := drv.Pay(echo.Context(nil), payCfg)
    if err != nil {
        panic(err)
    }
    println("跳转:", resp.RedirectURL)
}
```

示例：处理回调通知
----

```go
// 支付回调路由
r.POST("/pay/:platform/notify", func(c echo.Context) error {
    platform := c.Param("platform") // 如 "authorize", "razorpay" 等
    factory := payment.Get(platform)
    if factory == nil {
        return c.String("unsupported platform")
    }
    drv := factory()
    drv.SetAccount(account)   // 设置已配置的账户
    drv.SetNotifyCallback(callbackHandler) // 设置业务回调
    return drv.PayNotify(c)               // 内部完成验签+业务处理
})

// 退款回调路由（独立入口）
r.POST("/refund/:platform/notify", func(c echo.Context) error {
    platform := c.Param("platform")
    factory := payment.Get(platform)
    if factory == nil {
        return c.String("unsupported platform")
    }
    drv := factory()
    drv.SetAccount(account)
    drv.SetNotifyCallback(callbackHandler)
    return drv.RefundNotify(c)            // 退款专用回调处理
})
```
