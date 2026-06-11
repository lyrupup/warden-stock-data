# 守望者行情数据服务 · 开放 API 接入说明（第三方接入指南）

> Warden Stock Data Service · Open API Integration Guide
>
> 适用对象：接入本服务**只读行情数据**的第三方应用（交易系统 / 量化系统 / 数据看板等）。
> 配套文档：[`PRD.md`](./PRD.md) · [`BACKEND.md`](./BACKEND.md) · [`openapi.yaml`](./openapi.yaml)（机器可读契约，以其为准）。

---

## 1. 概述

本服务是一套独立的 **A 股行情数据中台**，对外提供一组**纯只读**的标准化行情数据 HTTP 接口（指数 / 个股快照 / K 线 / 分时 / 技术指标 / 搜索 / 元数据）。

- **接入入口（BasePath）**：`/open/v1`，所有开放接口均挂在该路径下。
- **鉴权方式**：每个请求需携带由管理员分发的 `secretId` / `secretKey`，并以 **HMAC-SHA256 签名**方式校验（详见 [§3 鉴权与签名](#3-鉴权与签名secretkey-使用)）。
- **权限边界（铁律）**：接入方**只有数据读取权限，绝无任何写入 / 更新权限**。开放 API 路由组下**不存在任何 POST / PUT / DELETE 写接口**。
- **数据精度**：所有金额 / 价格 / 比率 / 指标值均序列化为**带引号的字符串**（高精度 decimal，如 `"10.5000"`），接入方需自行转 number 后再计算，避免浮点误差。

> ⚠️ 免责：本服务仅提供行情数据，不构成投资建议；数据可能因数据源延迟 / 异常而存在偏差（响应中以 `stale` 标记提示）。

---

## 2. 快速开始

接入分三步：

1. **获取凭证**：联系本服务管理员，在管理后台为你的应用创建一对凭证，你将一次性获得：
   - `secretId`（公开标识，形如 `AKIDxxxxxxxxxxxxxxxxxxxxxxxxxxxx`）
   - `secretKey`（私密密钥，**仅创建/轮换时明文返回一次，请务必妥善保存**，服务端不再可查）
2. **构造签名请求**：按 [§3](#3-鉴权与签名secretkey-使用) 规则对每个请求计算 HMAC 签名，放入请求头。
3. **调用接口**：向 `http://<服务地址>/open/v1/...` 发起 GET 请求，解析统一响应结构。

最小示例（获取大盘指数）：

```bash
# 见 §3.4 的签名脚本生成 X-Signature 等头部后：
curl -s "http://localhost:8080/open/v1/indices?market=CN" \
  -H "X-Secret-Id: $SECRET_ID" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: $SIG"
```

---

## 3. 鉴权与签名（secretKey 使用）

开放 API 采用 **HMAC-SHA256 签名**鉴权，`secretKey` 仅参与**本地签名计算**，**绝不随请求明文传输**。每个请求都需重新计算签名。

### 3.1 必带请求头

| Header | 必填 | 说明 |
|--------|:---:|------|
| `X-Secret-Id` | ✅ | 凭证公开标识 `secretId` |
| `X-Timestamp` | ✅ | 请求 Unix **毫秒**时间戳（与服务器时间偏差需在 **±300 秒**内） |
| `X-Nonce` | ✅ | 一次性随机串（防重放，**300 秒**窗口内不可重复） |
| `X-Signature` | ✅ | 按下文规则计算的 HMAC-SHA256 签名（Base64 编码） |

### 3.2 签名串（StringToSign）构造

将以下 7 个字段用换行符 `\n` 拼接：

```
StringToSign = METHOD            + "\n" +   // 大写 HTTP 方法，如 GET
               PATH              + "\n" +   // 请求路径，如 /open/v1/indices
               CanonicalQuery    + "\n" +   // 见 §3.3
               X-Secret-Id       + "\n" +   // 与请求头一致
               X-Timestamp       + "\n" +   // 与请求头一致
               X-Nonce           + "\n" +   // 与请求头一致
               SHA256Hex(Body)              // 请求体的 SHA256 十六进制；GET 无 body 时为空串的 SHA256
```

### 3.3 CanonicalQuery（规范化查询串）

- 取本次请求的全部 query 参数，按**参数名（key）字典序升序排序**；
- 以 `key=value` 形式用 `&` 连接，如 `codes=600000,000001&market=CN`；
- 无 query 参数时为**空字符串**；
- value 使用**原始值**（与实际发送一致）。

### 3.4 计算签名

```
Signature = Base64( HMAC_SHA256(secretKey, StringToSign) )
```

将结果放入 `X-Signature` 头。

### 3.5 签名示例（Bash）

```bash
#!/usr/bin/env bash
# 依赖：openssl、coreutils
SECRET_ID="AKIDxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
SECRET_KEY="your-secret-key"
HOST="http://localhost:8080"

METHOD="GET"
PATH_="/open/v1/quotes"
QUERY="codes=600000,000001&market=CN"   # 已按 key 字典序排序
TS=$(($(date +%s) * 1000))
NONCE=$(openssl rand -hex 16)
BODY_HASH=$(printf '' | openssl dgst -sha256 -hex | awk '{print $2}')

STRING_TO_SIGN="${METHOD}
${PATH_}
${QUERY}
${SECRET_ID}
${TS}
${NONCE}
${BODY_HASH}"

SIG=$(printf '%s' "$STRING_TO_SIGN" \
  | openssl dgst -sha256 -hmac "$SECRET_KEY" -binary \
  | openssl base64 -A)

curl -s "${HOST}${PATH_}?${QUERY}" \
  -H "X-Secret-Id: ${SECRET_ID}" \
  -H "X-Timestamp: ${TS}" \
  -H "X-Nonce: ${NONCE}" \
  -H "X-Signature: ${SIG}"
```

### 3.6 签名示例（Node.js）

```javascript
import crypto from "node:crypto";

function buildHeaders({ secretId, secretKey, method, path, query = {}, body = "" }) {
  const ts = Date.now().toString();
  const nonce = crypto.randomBytes(16).toString("hex");
  const canonicalQuery = Object.keys(query)
    .sort()
    .map((k) => `${k}=${query[k]}`)
    .join("&");
  const bodyHash = crypto.createHash("sha256").update(body).digest("hex");
  const stringToSign = [method, path, canonicalQuery, secretId, ts, nonce, bodyHash].join("\n");
  const signature = crypto.createHmac("sha256", secretKey).update(stringToSign).digest("base64");
  return {
    "X-Secret-Id": secretId,
    "X-Timestamp": ts,
    "X-Nonce": nonce,
    "X-Signature": signature,
  };
}
```

### 3.7 签名示例（Python）

```python
import time, uuid, hashlib, hmac, base64

def build_headers(secret_id, secret_key, method, path, query=None, body=""):
    query = query or {}
    ts = str(int(time.time() * 1000))
    nonce = uuid.uuid4().hex
    canonical_query = "&".join(f"{k}={query[k]}" for k in sorted(query))
    body_hash = hashlib.sha256(body.encode()).hexdigest()
    string_to_sign = "\n".join([method, path, canonical_query, secret_id, ts, nonce, body_hash])
    signature = base64.b64encode(
        hmac.new(secret_key.encode(), string_to_sign.encode(), hashlib.sha256).digest()
    ).decode()
    return {
        "X-Secret-Id": secret_id,
        "X-Timestamp": ts,
        "X-Nonce": nonce,
        "X-Signature": signature,
    }
```

### 3.8 鉴权常见失败排查

| 错误码 | 含义 | 排查建议 |
|--------|------|---------|
| 41001 | 缺少凭证签名头 | 检查 4 个 `X-*` 头是否齐全 |
| 41002 | secretId 不存在 / 已吊销 / 已过期 | 确认 secretId 正确、凭证未被吊销、未过期 |
| 41003 | 签名校验失败 | 检查 StringToSign 拼接顺序、CanonicalQuery 排序、body 哈希是否一致 |
| 41004 | 时间戳过期 / nonce 重放 | 校准本机时钟（±300s）；每次请求使用全新 nonce |
| 41005 | 凭证 scope 不足 | 凭证仅 `read` 权限，请勿访问非只读资源 |

---

## 4. 通用约定

### 4.1 统一响应结构

所有接口返回如下 JSON 结构：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

- `code = 0` 表示成功；非 0 为业务错误码（见 [§6 错误码](#6-错误码)）。
- `data` 为业务数据，结构因接口而异（对象 / 数组 / 分页对象）。
- 分页接口 `data` 形如 `{ "list": [...], "total": 100, "page": 1, "size": 20 }`。

### 4.2 数值与时间

- **数值字段（价格 / 金额 / 量 / 比率 / 指标值）为 decimal 字符串**，如 `"10.5000"`；接入方须转 number 后计算。
- 时间字段为 ISO8601 字符串；日期字段形如 `"2026-06-05"`。

### 4.3 通用参数

- `market`：市场维度，V1 仅支持 `CN`（A 股），默认 `CN`。
- 所有接口支持 context 超时中断；数据源异常时部分接口降级返回快照并以 `stale=true` 标记。

---

## 5. 开放接口说明

> BasePath：`/open/v1`。以下所有接口均为 **GET**，均需 [§3](#3-鉴权与签名secretkey-使用) 的 HMAC 签名头。

接口总览：

| # | Method | URL | 说明 |
|---|--------|-----|------|
| 5.1 | GET | `/open/v1/indices` | 大盘指数 |
| 5.2 | GET | `/open/v1/quotes` | 批量个股快照 |
| 5.3 | GET | `/open/v1/stocks/{code}` | 单只个股快照 |
| 5.4 | GET | `/open/v1/stocks/{code}/kline` | 个股 K 线（可附带逐 bar 指标） |
| 5.5 | GET | `/open/v1/stocks/{code}/intraday` | 个股分时走势 |
| 5.6 | GET | `/open/v1/stocks/{code}/indicators` | 单只个股技术指标（实时计算） |
| 5.7 | GET | `/open/v1/indicators` | 批量个股技术指标（读快照，回测用） |
| 5.8 | GET | `/open/v1/search` | 股票搜索 |
| 5.9 | GET | `/open/v1/securities` | 证券列表 |
| 5.10 | GET | `/open/v1/meta` | 元数据（市场 / 指标目录 / 数据新鲜度） |

---

### 5.1 大盘指数

`GET /open/v1/indices`

返回当日大盘核心指数（上证指数、深证成指、创业板指、科创 50、沪深 300 等）。

**入参（query）**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|:---:|------|------|
| `market` | string | 否 | `CN` | 市场 |

**出参** `data`：`IndexQuote` 数组。

| 字段 | 类型 | 说明 |
|------|------|------|
| `market` | string | 市场 |
| `index_code` | string | 指数代码，如 `000001` |
| `index_name` | string | 指数名称，如 `上证指数` |
| `price` | decimal | 现价 |
| `change_amount` | decimal | 涨跌额 |
| `change_percent` | decimal | 涨跌幅（%） |
| `volume` | decimal | 成交量 |
| `amount` | decimal | 成交额 |
| `trade_date` | datetime | 交易时间 |

**示例响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "market": "CN",
      "index_code": "000001",
      "index_name": "上证指数",
      "price": "3050.1200",
      "change_amount": "12.3400",
      "change_percent": "0.4100",
      "volume": "28000000000",
      "amount": "350000000000.0000",
      "trade_date": "2026-06-05T15:00:00+08:00"
    }
  ]
}
```

---

### 5.2 批量个股快照

`GET /open/v1/quotes`

按代码批量返回个股盘口快照，命中缓存通常 < 1s。

**入参（query）**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|:---:|------|------|
| `codes` | string | ✅ | — | 逗号分隔代码，如 `600000,000001` |
| `market` | string | 否 | `CN` | 市场 |

**出参** `data`：`StockQuote` 数组（字段见 [5.3](#53-单只个股快照)）。

---

### 5.3 单只个股快照

`GET /open/v1/stocks/{code}`

**入参**

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|:---:|------|------|
| `code` | path | string | ✅ | — | 股票代码，如 `600000` |
| `market` | query | string | 否 | `CN` | 市场 |

**出参** `data`：`StockQuote`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `market` | string | 市场 |
| `stock_code` | string | 股票代码 |
| `stock_name` | string | 股票名称 |
| `price` | decimal | 现价 |
| `open` / `high` / `low` | decimal | 开 / 高 / 低 |
| `prev_close` | decimal | 昨收 |
| `change_percent` | decimal | 涨跌幅（%） |
| `volume` | decimal | 成交量 |
| `amount` | decimal | 成交额 |
| `turnover_rate` | decimal | 换手率（%） |
| `trade_date` | datetime | 交易时间 |
| `stale` | boolean | `true` 表示数据源异常时返回的降级快照 |

**说明**：源异常时返回最近快照并置 `stale=true`，不阻塞调用方。代码不存在返回 `404`。

---

### 5.4 个股 K 线

`GET /open/v1/stocks/{code}/kline`

返回个股日 / 周 / 月 / 分钟 K 线，支持复权口径与 `limit`/`offset` 分页或 `from`/`to` 区间查询；可选附带与 bars 对齐的逐 bar 技术指标。

**入参**

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|:---:|------|------|
| `code` | path | string | ✅ | — | 股票代码 |
| `period` | query | string | 否 | `day` | `day`/`week`/`month`/`1m`/`5m`/`15m`/`30m`/`60m` |
| `adjust` | query | string | 否 | `qfq` | 复权口径：`""`(不复权)/`qfq`(前复权)/`hfq`(后复权) |
| `limit` | query | int | 否 | `120` | 每页根数；与 `from`/`to` 互斥，区间优先 |
| `offset` | query | int | 否 | `0` | 自最新交易日向历史跳过的根数，配合 `limit` 分页 |
| `from` | query | date | 否 | — | 起始交易日（含），回测区间用 |
| `to` | query | date | 否 | — | 结束交易日（含），回测区间用 |
| `indicators` | query | string | 否 | — | 逗号分隔指标类型（取值见 `/open/v1/meta`）；传入则返回对象结构 |
| `market` | query | string | 否 | `CN` | 市场 |

**出参**

- **不带 `indicators`**：`data` 为 `Kline` 数组：

  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `date` | string | 交易日，如 `2026-06-05` |
  | `open` / `high` / `low` / `close` | decimal | OHLC |
  | `volume` | decimal | 成交量 |
  | `amount` | decimal | 成交额 |

- **带 `indicators`**：`data` 为对象 `{ bars, indicators, has_more }`：
  - `bars`：`Kline` 数组（同上）；
  - `indicators`：`IndicatorResult` 数组，与 `bars` 按 `trade_date` 对齐（point-in-time，无未来函数）；
  - `has_more`：boolean，窗口左侧（更早方向）是否还有可分页加载的历史 K 线。

**说明**：带 `indicators` 时采用「快照优先 + 实时补齐」——日 K + 前复权且指标在默认快照集合内 → 读快照；周/月 K、后复权、非默认指标（如 `ma120`）或快照缺口 → 实时逐 bar 计算。

---

### 5.5 个股分时走势

`GET /open/v1/stocks/{code}/intraday`

返回个股**分时数据**（逐交易分钟的价格、均价、成交量 + 昨收基准），实时透传不落库。

**入参**

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|:---:|------|------|
| `code` | path | string | ✅ | — | 股票代码 |
| `market` | query | string | 否 | `CN` | 市场 |

**出参** `data`：`Intraday`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `market` | string | 市场 |
| `stock_code` | string | 股票代码 |
| `stock_name` | string | 股票名称 |
| `trade_date` | string | 实际数据所属交易日 |
| `pre_close` | decimal | 昨收基准 |
| `points` | array | 分时点数组 |
| `points[].time` | datetime | 分钟时间点 |
| `points[].price` | decimal | 价格 |
| `points[].avg_price` | decimal | 均价 |
| `points[].volume` | decimal | 该分钟成交量 |

**说明**：非交易日 / 盘前当日无分时时**自动回退到最近交易日**，响应 `trade_date` 标注实际数据所属交易日。

---

### 5.6 单只个股技术指标（实时计算）

`GET /open/v1/stocks/{code}/indicators`

按代码实时计算并返回指定技术指标，缺失自动回退。

**入参**

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|:---:|------|------|
| `code` | path | string | ✅ | — | 股票代码 |
| `types` | query | string | 否 | `ma5,ma10,ma20,ma30,ma60` | 逗号分隔指标类型，取值见 `/open/v1/meta` |
| `market` | query | string | 否 | `CN` | 市场 |

**出参** `data`：`IndicatorResult`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `stock_code` | string | 股票代码 |
| `trade_date` | date | 指标所属交易日 |
| `values` | object | `指标类型 → decimal 值` 的映射 |

**可用指标类型**（详见 `/open/v1/meta` 的 `indicators` 目录）：

- 均线：`ma5` `ma10` `ma20` `ma30` `ma60`
- 乖离：`bias5` `bias10` `bias20`；均线多头排列 `ma_align`
- 振幅：`amplitude` `amplitude_streak`
- 动量：`pct_change` `pct_change5` `pct_change20` `pct_change60`
- 量比：`vol_ratio`
- MACD：`macd_dif` `macd_dea` `macd_bar`（12/26/9）
- KDJ：`kdj_k` `kdj_d` `kdj_j`（9/3/3）
- RSI：`rsi6` `rsi12` `rsi24`
- BOLL：`boll_mid` `boll_upper` `boll_lower`（20/2）
- ATR：`atr14` `atr20`

**示例响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "stock_code": "600000",
    "trade_date": "2026-06-05",
    "values": { "ma5": "12.3400", "ma20": "11.8000", "macd_bar": "0.1500", "rsi6": "65.0000" }
  }
}
```

---

### 5.7 批量个股技术指标（读快照，回测用）

`GET /open/v1/indicators`

读取盘后全市场预计算的历史指标快照。指定 `trade_date` 即取该交易日的 **point-in-time** 指标（无未来函数），适合按日回放的量化回测信号生成。

**入参**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|:---:|------|------|
| `codes` | string | ✅ | — | 逗号分隔代码 |
| `types` | string | 否 | `ma5,ma10,ma20,ma30,ma60` | 逗号分隔指标类型 |
| `trade_date` | date | 否 | — | 指定历史交易日（point-in-time）；为空取最新交易日 |
| `market` | string | 否 | `CN` | 市场 |

**出参** `data`：`IndicatorResult` 数组（结构见 [5.6](#56-单只个股技术指标实时计算)）。

**说明**：盘后全市场扫描默认落库的指标集合为 `ma5/10/20/30/60`、`macd_dif/dea/bar`、`kdj_k/d/j`、`rsi6/12/24`、`boll_mid/upper/lower`、`atr14/atr20`、`pct_change20/60`；其余指标（如 `bias`、`vol_ratio`）请经 [5.6](#56-单只个股技术指标实时计算) 单只实时接口按需计算。可经 `/open/v1/meta` 的 `default_snapshot_types` 确认哪些指标可批量按日读取。

---

### 5.8 股票搜索

`GET /open/v1/search`

按代码 / 名称关键字搜索标的。

**入参**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|:---:|------|------|
| `kw` | string | ✅ | — | 关键字（代码或名称） |
| `market` | string | 否 | `CN` | 市场 |

**出参** `data`：`StockBrief` 数组。

| 字段 | 类型 | 说明 |
|------|------|------|
| `stock_code` | string | 股票代码 |
| `stock_name` | string | 股票名称 |
| `market` | string | 市场 |
| `board` | string | 板块，如 `主板` |

---

### 5.9 证券列表

`GET /open/v1/securities`

返回全市场证券列表。

**入参**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|:---:|------|------|
| `market` | string | 否 | `CN` | 市场 |

**出参** `data`：`StockBrief` 数组（结构见 [5.8](#58-股票搜索)）。

---

### 5.10 元数据

`GET /open/v1/meta`

接入方发现服务能力的统一入口：市场列表、指标目录、默认快照指标集合、数据新鲜度。

**入参**：无。

**出参** `data`：`Meta`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `markets` | array | 支持的市场列表（`code`/`name`/`enabled`） |
| `indicators` | array | 指标目录（动态派生，注册即可见） |
| `indicators[].type` | string | 指标类型，如 `macd_bar` |
| `indicators[].name` | string | 指标名称 |
| `indicators[].group` | string | 分类，如 `MACD` |
| `indicators[].value_type` | string | `number` / `bool` |
| `indicators[].snapshot` | boolean | 是否进默认逐日快照（`true` 才能经 `/open/v1/indicators` 批量按日读取） |
| `indicators[].implemented` | boolean | 是否已实现（恒 `true`） |
| `indicators[].params` | object | 计算参数（口径），如 `{fast:12,slow:26,signal:9}` |
| `default_snapshot_types` | array | 可批量按交易日 point-in-time 读取的指标类型（回测构建策略关键依据） |
| `freshness` | object | 数据新鲜度（最新交易日 / K 线更新至 / 证券数等） |

---

## 6. 错误码

| 错误码 | HTTP | 含义 |
|--------|------|------|
| 0 | 200 | 成功 |
| 10001 | 400 | 参数错误 |
| 10002 | 404 | 资源不存在 |
| 10408 | 408 | 请求超时（context 取消） |
| 20001 | 200 | 行情数据源异常（已降级返回 stale 快照） |
| 20002 | 404 | 股票不存在 |
| 21001 | 400 | 指标参数非法 |
| 41001 | 401 | 缺少凭证签名头 |
| 41002 | 401 | secretId 不存在 / 已吊销 / 已过期 |
| 41003 | 401 | 签名校验失败 |
| 41004 | 401 | 时间戳过期 / nonce 重放 |
| 41005 | 401 | 凭证 scope 不足（试图访问非只读资源） |
| 42001 | 429 | 触发限流（QPS） |
| 42002 | 429 | 超出日调用配额 |

---

## 7. 限流与配额

- 每对凭证可配置 **QPS 限流**与**日调用配额**，由管理员分发时设定。
- 触发 QPS 限流返回 `42001`，超出日配额返回 `42002`，均为 HTTP `429`。
- 建议接入方实现**指数退避重试**，并合理利用本地缓存降低调用频次。

---

## 8. 最佳实践

- **时钟同步**：保持服务器时间准确（NTP），避免 `41004` 时间戳过期。
- **nonce 唯一**：每次请求生成全新随机串，勿复用。
- **decimal 安全**：数值字段先转 number 再计算，禁止直接对字符串做算术。
- **降级容错**：识别 `stale=true`，对降级数据按业务需要提示或重试。
- **能力发现**：接入前先调 `/open/v1/meta` 了解支持的市场、指标目录与可回放指标集合。
- **只读约束**：本服务对接入方零写权限；任何更新由管理员后台触发。

---

> 本文档为人读接入指南，机器可读契约以 [`openapi.yaml`](./openapi.yaml) 为准；二者如有出入，以 `openapi.yaml` 为权威。
