# Aeonblight 对外接口列表

版本日期：2026-07-14  
代码基线：123 条已注册 HTTP 路由中的 85 条非管理员路由  
不包含：`admin-api` 管理接口；请参阅 [管理员接口文档](admin-interface-reference.md)

## 服务地址

| 服务 | 本地默认地址 | 路由数 | 说明 |
| --- | --- | ---: | --- |
| `account-api` | `http://127.0.0.1:8081` | 23 | 钱包登录、Session、角色、游戏服、Launch Ticket、在线状态、副本恢复 |
| `economy-api` | `http://127.0.0.1:8082` | 62 | 经济快照、库存、装备、副本、活动、交易、支付、链上结算和 Worker 命令 |

完整字段、示例和错误说明见 [接口详解](interface-reference.md)。

## 认证标记

| 标记 | 含义 |
| --- | --- |
| 公开 | 无认证；仍受请求格式和业务校验约束 |
| JWT | `Authorization: Bearer <accessToken>` |
| `account.gameplay` | 游戏服 Service Identity 签名 |
| `account.ops` | 账号运维 Service Identity 签名 |
| `economy.gameplay` | 游戏服 Service Identity 签名 |
| `economy.worker` | Economy Worker Service Identity 签名 |
| `economy.payments` | 链支付操作器 Service Identity 签名 |
| `economy.mint` | NFT Mint 操作器 Service Identity 签名 |
| `economy.boss_ops` | Boss 运维 Service Identity 签名 |
| `economy.rewards` | 奖励发放操作器 Service Identity 签名 |

> `development/test` 可使用 `X-Internal-Key` 兼容模式；`staging/production` 必须使用独立 Ed25519 Service Identity。

## account-api（23）

### 健康与认证

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 1 | GET | `/health` | 公开 | 进程存活检查 |
| 2 | GET | `/ready` | 公开 | PostgreSQL、Schema、Redis 等依赖就绪检查 |
| 3 | GET | `/api/auth/wallet/nonce` | 公开 | 获取 Solana 钱包登录 nonce 和待签消息 |
| 4 | POST | `/api/auth/wallet` | 公开 | 验证钱包签名并创建 Session |
| 5 | POST | `/api/auth/refresh` | 公开 | 轮换 Refresh Token 并签发新 Access Token |
| 6 | POST | `/api/auth/logout` | JWT | 注销 Session |
| 7 | GET | `/api/auth/verify` | JWT | 验证 Access Token 并返回账号 |
| 8 | GET | `/api/auth/session/redis` | `account.ops` | 查看 Session Redis 运行状态 |

### 角色与启动

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 9 | GET | `/api/character/list` | `account.gameplay` | 查询账号角色列表 |
| 10 | POST | `/api/character/create` | `account.gameplay` | 创建角色 |
| 11 | POST | `/api/game/launch` | JWT | 创建指定游戏服的短期 Launch Ticket |
| 12 | GET | `/api/game/dungeon/recovery` | JWT | 主页检查是否必须恢复未完成副本 |
| 13 | POST | `/api/game/dungeon/recovery` | JWT | 选择恢复原服或放弃副本 |
| 14 | POST | `/api/game/launch/consume` | `account.gameplay` | 原游戏服消费 Launch Ticket 并建立在线状态 |

### 游戏服与在线状态

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 15 | POST | `/api/game/servers/register` | `account.gameplay` | 注册或更新当前游戏服 |
| 16 | POST | `/api/game/servers/heartbeat` | `account.gameplay` | 上报游戏服心跳和在线人数 |
| 17 | GET | `/api/game/servers` | `account.gameplay` | 查询游戏服列表 |
| 18 | POST | `/api/game/online/enter` | `account.gameplay` | 建立或迁移玩家 Online Presence |
| 19 | POST | `/api/game/online/heartbeat` | `account.gameplay` | 刷新玩家在线状态 |
| 20 | POST | `/api/game/online/leave` | `account.gameplay` | 移除玩家在线状态 |
| 21 | GET | `/api/game/online` | `account.gameplay` | 查询单个账号的在线状态 |
| 22 | GET | `/api/game/online/server` | `account.gameplay` | 查询当前游戏服的在线账号 |
| 23 | POST | `/api/game/online/sweep` | `account.ops` | 清理过期 Online Presence |

## economy-api（62）

### 健康与经济快照

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 1 | GET | `/health` | 公开 | 进程存活检查 |
| 2 | GET | `/ready` | 公开 | PostgreSQL、Schema、经济配置、Solana RPC 就绪检查 |
| 3 | GET | `/api/economy/snapshot` | `economy.gameplay` | 获取角色完整经济快照 |

### 仓库、装备与 NFT

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 4 | POST | `/api/economy/warehouse/deposit` | `economy.gameplay` | 背包物品/装备存入仓库 |
| 5 | POST | `/api/economy/warehouse/withdraw` | `economy.gameplay` | 仓库物品/装备取回背包 |
| 6 | POST | `/api/economy/equipment/equip` | `economy.gameplay` | 穿戴装备 |
| 7 | POST | `/api/economy/equipment/unequip` | `economy.gameplay` | 卸下装备 |
| 8 | POST | `/api/economy/equipment/repair` | `economy.gameplay` | 使用 AEB 修复装备耐久 |
| 9 | POST | `/api/economy/equipment/enhance` | `economy.gameplay` | 强化一个随机副词条实例（最高 +10） |
| 10 | POST | `/api/economy/equipment/npc-recycle` | `economy.gameplay` | 向 NPC 回收非 NFT 装备，保留 7 天恢复窗口 |
| 11 | POST | `/api/economy/lottery/draw` | `economy.gameplay` | 创建 30 AEB/次的抽奖 Payment Order，并快照开奖计划 |
| 12 | GET | `/api/economy/bounty/board` | `economy.gameplay` | 返回并补齐角色已解锁的悬赏槽位 |
| 13 | POST | `/api/economy/bounty/slots/unlock-gold` | `economy.gameplay` | 使用 Gold 解锁角色槽位 2 |
| 14 | POST | `/api/economy/bounty/slots/unlock-aeb` | `economy.gameplay` | 创建账号共享槽位 3–5 的 AEB Payment Order |
| 15 | POST | `/api/economy/bounty/refresh` | `economy.gameplay` | 免费、Gold 或 Premium 刷新可替换的普通任务 |
| 16 | POST | `/api/economy/bounty/progress/combat` | `economy.gameplay` | 游戏服提交已完成副本的去重战斗进度 |
| 17 | POST | `/api/economy/bounty/submit-equipment` | `economy.gameplay` | 消耗合格背包装备完成稀有任务 |
| 18 | POST | `/api/economy/bounty/claim` | `economy.gameplay` | 领取已完成任务的徽章 |
| 19 | POST | `/api/economy/bounty/badges/draw` | `economy.gameplay` | 消耗悬赏徽章并发放奖励 |
| 20 | POST | `/api/economy/nft/mint/request` | `economy.gameplay` | 创建装备 NFT Mint 请求并扣费/锁定装备 |
| 21 | POST | `/api/economy/nft/mint/cancel` | `economy.gameplay` | 取消 Mint 请求并按原余额类别退款 |
| 22 | POST | `/api/economy/internal/nft/mint/confirm` | `economy.mint` | 确认 Mint 结果；生产当前 fail-closed |
| 23 | GET | `/api/economy/nft/assets` | `economy.gameplay` | 查询账号 NFT Asset |

### 副本、掉落与活动

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 24 | POST | `/api/economy/dungeon/enter` | `economy.gameplay` | 创建绑定原游戏服的 Dungeon Run |
| 25 | POST | `/api/economy/dungeon/finish` | `economy.gameplay` | 原游戏服结束副本并结算 |
| 26 | POST | `/api/economy/loot/claim-player` | `economy.gameplay` | 领取一项 Loot Tray 奖励 |
| 27 | POST | `/api/economy/loot/claim-all` | `economy.gameplay` | 批量领取 Loot Tray 奖励 |
| 28 | POST | `/api/economy/loot/discard` | `economy.gameplay` | 丢弃 Loot Tray 奖励 |
| 29 | POST | `/api/economy/gathering/settle` | `economy.gameplay` | 结算采集节点 |
| 30 | POST | `/api/economy/farming/harvest` | `economy.gameplay` | 结算农作物收获 |
| 31 | POST | `/api/economy/boss/contribute` | `economy.gameplay` | 累加玩家 Boss 贡献 |
| 32 | POST | `/api/economy/boss/settle` | `economy.gameplay` | 结算玩家 Boss 奖励 |
| 33 | POST | `/api/economy/internal/boss/events/open` | `economy.boss_ops` | 开启 Boss Event |
| 34 | POST | `/api/economy/internal/boss/events/close` | `economy.boss_ops` | 关闭 Boss Event |
| 35 | POST | `/api/economy/internal/boss/events/settle` | `economy.boss_ops` | 标记 Boss Event 已结算 |
| 36 | GET | `/api/economy/internal/boss/events/active` | `economy.boss_ops` | 查询活动中的 Boss Event |

### 背包与成长购买

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 37 | POST | `/api/economy/inventory/organize` | `economy.gameplay` | 整理背包 |
| 38 | POST | `/api/economy/warehouse/organize` | `economy.gameplay` | 整理仓库 |
| 39 | POST | `/api/economy/inventory/discard` | `economy.gameplay` | 丢弃背包物品/装备 |
| 40 | POST | `/api/economy/inventory/synthesize` | `economy.gameplay` | 按 Recipe 合成 |
| 41 | POST | `/api/economy/inventory/bag/expand` | `economy.gameplay` | 创建背包扩容 Payment Order |
| 42 | POST | `/api/economy/license/purchase` | `economy.gameplay` | 创建交易许可证 Payment Order |

### Marketplace 与支付

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 43 | GET | `/api/economy/marketplace/listings` | `economy.gameplay` | 查询市场挂单 |
| 44 | GET | `/api/economy/marketplace/listings/mine` | `economy.gameplay` | 查询自己的挂单 |
| 45 | GET | `/api/economy/marketplace/slots` | `economy.gameplay` | 查询挂单槽位 |
| 46 | POST | `/api/economy/marketplace/list` | `economy.gameplay` | 创建物品/装备挂单 |
| 47 | POST | `/api/economy/marketplace/listings/{listingId}/buy` | `economy.gameplay` | 购买挂单 |
| 48 | POST | `/api/economy/marketplace/listings/{listingId}/cancel` | `economy.gameplay` | 取消自己的挂单 |
| 49 | POST | `/api/economy/marketplace/slots/expand-material` | `economy.gameplay` | 使用材料扩展挂单槽位 |
| 50 | POST | `/api/economy/marketplace/slots/expand-wallet` | `economy.gameplay` | 创建链上付款扩槽订单 |
| 51 | POST | `/api/economy/marketplace/slots/expand-wallet/submit` | `economy.gameplay` | 玩家提交链上付款签名 |
| 52 | POST | `/api/economy/internal/payments/submit` | `economy.payments` | 支付操作器提交并验证付款签名 |

### Token、Worker 与链结算

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 53 | POST | `/api/economy/rewards/grant-locked` | `economy.rewards` | 发放带冷却期的 Locked AEB |
| 54 | POST | `/api/chain/token/claim` | `economy.gameplay` | 申请 AEB 提现 |
| 55 | GET | `/api/chain/token/ledger` | `economy.gameplay` | 查询账号经济账本 |
| 56 | POST | `/api/economy/internal/unlocks/settle` | `economy.worker` | 解锁到期 Locked AEB |
| 57 | POST | `/api/economy/internal/withdrawals/process` | `economy.worker` | 推进提现队列 |
| 58 | POST | `/api/economy/internal/chain/deposits/scan` | `economy.worker` | 分页扫描 Solana 充值 |
| 59 | POST | `/api/economy/internal/chain/payouts/submit` | `economy.worker` | 提交待处理 Solana Payout |
| 60 | POST | `/api/economy/internal/chain/payouts/confirm` | `economy.worker` | 确认 Solana Payout |
| 61 | POST | `/api/economy/internal/payments/confirm` | `economy.payments` | 确认已提交 Payment Order |
| 62 | POST | `/api/economy/internal/equipment/npc-recycle/purge` | `economy.worker` | 物理删除已过 7 天恢复期的 NPC 回收装备 |
