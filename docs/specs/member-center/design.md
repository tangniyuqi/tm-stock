# 个人中心 · 设计

## 1. 方案概述

手机号验证码登录 + JWT；会员状态与试吃计数**全部服务端判定**；
订阅通过**兑换码**激活，**支付完全发生在 App 之外**。

## 2. 页面规格（`pages/member/index`）

```
┌─────────────────────────────┐
│ 138****8000                 │  ← 脱敏
├─────────────────────────────┤
│ 会员：有效期至 2026-12-31     │
│ 今日剩余试吃：2 次            │
├─────────────────────────────┤
│ 🎟  输入兑换码            >  │
│ ❓  如何开通              >  │  ← 纯文字说明，无支付入口
├─────────────────────────────┤
│ 📄 用户协议               >  │
│ 🔒 隐私政策               >  │
├─────────────────────────────┤
│    退出登录                  │
│    注销账号                  │  ← 弱化但必须可达（PIPL）
├─────────────────────────────┤
│ 本平台为公开信息聚合与历史数据  │
│ 统计，不构成任何投资建议。      │  ← 常驻
└─────────────────────────────┘
```

**登录页**：手机号 + 验证码 +
`☐ 我已阅读并同意《用户协议》《隐私政策》`（**默认不勾选**）。

## 3. API 契约

> 统一 `{code,msg,data}`；时间字段**毫秒时间戳 int64**。

### `POST /api/v1/auth/sms-code`
入参 `{ phone }`；限流：同手机号 60s 一次、同 IP 每小时 10 次。鉴权：否

### `POST /api/v1/auth/login`
入参 `{ phone, code, agreed: bool }`；
**`agreed` 必须为 true，否则 400**（同意留痕，见 §5）。鉴权：否
```
LoginResp { token string, expiresAt int64, isNewUser bool }
```

### `GET /api/v1/member/profile`
鉴权：是
```
Profile {
  phoneMasked  string   // 138****8000，服务端脱敏，不返回明文
  memberStatus string   // NONE | ACTIVE | EXPIRED
  expireAt     int64 | null
  trialLeft    int
  trialResetAt int64    // 下次重置时点（GMT+8 次日 0 点）
}
```

### `POST /api/v1/member/redeem`
入参 `{ code }`。鉴权：是
```
RedeemResp { expireAt int64 }
```
失败错误码需可区分（见下）。

### `POST /api/v1/member/deactivate`
入参 `{ confirmed: bool }`。鉴权：是
执行手机号匿名化 + 登录态失效。

### 错误码
| 码 | 含义 |
|---|------|
| 40010 | 未同意协议 |
| 40021 | 验证码错误或已过期 |
| 40022 | 验证码获取过于频繁 |
| 40031 | 兑换码不存在 |
| 40032 | 兑换码已被使用 |
| 40033 | 兑换码已过期 |
| 40034 | 兑换码不适用于当前账号 |
| 40035 | 错误次数过多，已临时锁定 |

> ⚠️ 四种兑换码失败**必须分开返回**。合并成"兑换码无效"会让客服无法定位问题，
> 最后变成人工查库。

### 🔴 契约层面的合规约束

1. **不得存在**任何支付相关接口（下单、支付回调、退款）。
2. 任何接口**不得返回**用户明文手机号——脱敏在**服务端**完成。
3. **不得存在**收益/战绩/胜率类字段。

## 4. 数据模型

```sql
-- migrations/20260730_create_member_tables.sql

CREATE TABLE member (
  id           BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone_hash   CHAR(64)    NOT NULL COMMENT '手机号 HMAC，用于查重与登录匹配',
  phone_enc    VARBINARY(256) NULL  COMMENT '手机号密文；注销后置 NULL',
  phone_masked VARCHAR(16) NOT NULL COMMENT '138****8000，展示用',
  status       VARCHAR(16) NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE|DEACTIVATED',
  expire_at    DATETIME    NULL COMMENT '会员到期时间；NULL 表示从未开通',
  created_at   DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  UNIQUE KEY uk_phone_hash (phone_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE member_consent (          -- PIPL：同意留痕
  id          BIGINT PRIMARY KEY AUTO_INCREMENT,
  member_id   BIGINT NOT NULL,
  policy_ver  VARCHAR(32) NOT NULL COMMENT '同意时的政策版本号',
  agreed_at   DATETIME NOT NULL,
  ip          VARCHAR(64) NOT NULL DEFAULT '',
  KEY idx_member (member_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT '🔴 举证材料：证明用户在何时同意了哪一版政策';

CREATE TABLE redeem_code (
  code        VARCHAR(32) PRIMARY KEY,
  days        INT         NOT NULL COMMENT '可延长天数',
  bind_phone_hash CHAR(64) NULL COMMENT '非空则仅限该手机号使用',
  status      VARCHAR(16) NOT NULL DEFAULT 'UNUSED' COMMENT 'UNUSED|USED|VOID',
  used_by     BIGINT      NULL,
  used_at     DATETIME    NULL,
  expire_at   DATETIME    NOT NULL COMMENT '码本身的有效期',
  order_ref   VARCHAR(64) NOT NULL DEFAULT '' COMMENT '站外订单号，用于对账追溯',
  created_by  VARCHAR(64) NOT NULL,
  created_at  DATETIME NOT NULL,
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE member_trial (            -- 试吃计数（按自然日）
  member_id BIGINT NOT NULL,
  biz_date  DATE   NOT NULL,
  theme_ids TEXT   NOT NULL COMMENT '当日已试吃的题材 id 集合（同题材只扣一次）',
  used      INT    NOT NULL DEFAULT 0,
  PRIMARY KEY (member_id, biz_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**手机号存储三件套**：
- `phone_hash`（HMAC，带盐）→ 登录匹配与唯一性，**不可逆**
- `phone_enc`（可逆加密）→ 仅在必须触达用户时解密（如发送短信）
- `phone_masked` → 展示

**注销 = 把 `phone_enc` 置 NULL + `status=DEACTIVATED`**。
`phone_hash` 保留以防同号重复注册冲突，但它不可逆，**不构成个人信息留存**。

## 5. 同意留痕设计（PIPL 举证）

隐私政策有**版本号**。用户同意时记录 `policy_ver`。
政策更新后，老用户下次登录需**重新同意新版本**。

> 没有留痕，"用户同意过"就只是口头说法。这张表是被问询时唯一能拿出来的东西。

## 6. 兑换码流程与对账

```
站外付款 → 客服在运营后台生成码（填 order_ref = 站外订单号）
        → 发码给用户 → 用户 App 内激活 → used_by / used_at 落库
```

- `order_ref` 是**对账钥匙**：任何一笔收入都能追到具体激活账号
- `bind_phone_hash` 可选：需要防转卖时绑定手机号
- 码本身有 `expire_at`，防止长期未用的码堆积
- **生成能力在运营后台**（`admin-console` 需补一个模块，见任务表）

## 7. 关键取舍

| 取舍 | 决定 | 理由 |
|-----|------|------|
| 支付方式 | **兑换码，支付全在体外** | 一期不做在线支付 → 免经营性 ICP |
| 说明页形式 | **纯文字**，无收款码无外链 | 内嵌收款码 = 事实上的收银台 |
| 手机号存储 | hash + 加密 + 脱敏 三件套 | 兼顾登录匹配、必要触达、PIPL 最小化 |
| 注销实现 | **匿名化**，非软删标记 | 打 `deleted` 标记不满足 PIPL |
| 同意 | 记版本号 + 时间 + IP | 无留痕 = 无法举证 |
| 兑换码失败提示 | **四种分开** | 合并成"无效"会把问题推给人工查库 |
| 第三方登录 | 一期不做 | 需额外资质与 PIPL 条款，收益不抵成本 |

## 8. 风险与回退

- **风险 1（最大）**：**兑换码是否足以规避"经营性"认定，未经律师确认。**
  若被认定仍需经营性 ICP，回退方案是继续走兑换码但补办许可
  （**不是**改回在线支付）。
- **风险 2**：隐私政策与用户协议**必须律师审阅**，不能自己拼。
  文本未就位前**不得开放注册**——一旦收了手机号就已触发义务。
- **风险 3**：短信服务需实名与签名报备，有前置周期。
- **风险 4**：兑换码人工发放在量大时成为瓶颈 → 二期考虑自动化，
  但**自动化也不得把支付搬进 App**。
