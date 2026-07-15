# Aeonblight 对外接口列表

版本日期：2026-07-15  
代码基线：157 条已注册 HTTP 路由中的 99 条非管理员路由  
不包含：`admin-api` 管理接口；请参阅 [管理员接口文档](admin-interface-reference.md)  
详情、入参和示例见 [接口详解](interface-reference.md)。

## 服务地址

| 服务 | 本地默认地址 | 路由数 | 主要调用方 |
| --- | --- | ---: | --- |
| `account-api` | `http://127.0.0.1:8081` | 32 | 钱包登录、启动器、游戏服、账号侧运维 |
| `economy-api` | `http://127.0.0.1:8082` | 67 | 游戏服、支付操作器、Worker、奖励发放器 |

## 认证标记

| 标记 | 说明 |
| --- | --- |
| 公开 | 无认证；仍会校验请求格式、参数和业务状态 |
| JWT | 玩家登录后获得的 `Authorization: Bearer <accessToken>` |
| `account.gameplay` | 可信游戏服调用账号侧 gameplay 接口 |
| `account.ops` | 账号侧运维任务 |
| `economy.gameplay` | 可信游戏服调用经济侧 gameplay 接口 |
| `economy.worker` | 后台 Worker 定时任务 |
| `economy.payments` | 链上付款提交、确认操作器 |
| `economy.mint` | NFT Mint 操作器 |
| `economy.boss_ops` | Boss 活动运维操作器 |
| `economy.rewards` | 奖励发放操作器 |

> `development/test` 可使用 `X-Internal-Key` 兼容模式；`staging/production` 必须使用独立 Ed25519 Service Identity。

## account-api（32）

### 基础、钱包登录与 Session

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 1 | GET | `/health` | 公开 | 进程存活检查 |
| 2 | GET | `/ready` | 公开 | PostgreSQL、Schema、Redis 等依赖就绪检查 |
| 3 | GET | `/api/auth/wallet/nonce` | 公开 | 获取钱包登录 nonce 和待签消息 |
| 4 | POST | `/api/auth/wallet` | 公开 | 验证钱包签名，创建账号 Session |
| 5 | POST | `/api/auth/refresh` | 公开 | 轮换 Refresh Token 并签发新 Access Token |
| 6 | POST | `/api/auth/logout` | JWT | 注销 Session 或 Refresh Token |
| 7 | GET | `/api/auth/verify` | JWT | 验证 Access Token 并返回账号信息 |
| 8 | GET | `/api/auth/session/redis` | `account.ops` | 查看 Session Redis/PostgreSQL 运行模式 |

### 公开主页、选服与排行榜

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 9 | GET | `/api/public/servers` | 公开 | 启动器选服列表，不暴露连接地址 |
| 10 | GET | `/api/public/servers/online` | 公开 | 只返回可进入的在线服务器 |
| 11 | GET | `/api/public/home/stats` | 公开 | 主页在线人数和 30 天活跃账号数 |
| 12 | GET | `/api/public/home/config` | 公开 | 主页公开 Token、客户端地址和钱包配置 |
| 13 | GET | `/api/public/leaderboards/clear-progress` | 公开 | 永久通关进度榜 |
| 14 | GET | `/api/public/leaderboards/weekly-score` | 公开 | 7 天游积分榜 |

### 角色、存档与启动流程

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 15 | GET | `/api/character/list` | `account.gameplay` | 查询角色槽位、登录外观和已穿戴装备 |
| 16 | POST | `/api/character/create` | `account.gameplay` | 创建角色并写入初始外观 |
| 17 | POST | `/api/character/delete` | `account.gameplay` | 软删除角色并释放角色槽 |
| 18 | GET | `/api/player/profile` | `account.gameplay` | 获取玩家运行态存档和经济快照 |
| 19 | POST | `/api/player/save` | `account.gameplay` | 保存位置、地图、饥饿、游玩时长等非经济运行态 |
| 20 | POST | `/api/game/launch` | JWT | 创建账号级、服务器绑定的短期 Launch Ticket |
| 21 | GET | `/api/game/dungeon/recovery` | JWT | 选角后检查是否必须恢复未完成副本 |
| 22 | POST | `/api/game/dungeon/recovery` | JWT | 恢复原服副本或放弃副本 |
| 23 | POST | `/api/game/launch/consume` | `account.gameplay` | 原游戏服消费 Launch Ticket，获得账号 admission |

### 游戏服注册与在线状态

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 24 | POST | `/api/game/servers/register` | `account.gameplay` | 注册或更新当前游戏服 |
| 25 | POST | `/api/game/servers/heartbeat` | `account.gameplay` | 上报游戏服心跳和在线人数 |
| 26 | GET | `/api/game/servers` | `account.gameplay` | 游戏服内部查询详细服务器列表 |
| 27 | POST | `/api/game/online/enter` | `account.gameplay` | 客户端选角后建立或迁移 Online Presence |
| 28 | POST | `/api/game/online/heartbeat` | `account.gameplay` | 刷新玩家在线状态 |
| 29 | POST | `/api/game/online/leave` | `account.gameplay` | 移除玩家在线状态 |
| 30 | GET | `/api/game/online` | `account.gameplay` | 查询单个账号的在线状态 |
| 31 | GET | `/api/game/online/server` | `account.gameplay` | 查询当前游戏服的在线账号 |
| 32 | POST | `/api/game/online/sweep` | `account.ops` | 清理过期 Online Presence |

## economy-api（67）

### 基础、公告与经济快照

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 1 | GET | `/health` | 公开 | 进程存活检查 |
| 2 | GET | `/ready` | 公开 | PostgreSQL、Schema、经济配置、Solana RPC 就绪检查 |
| 3 | GET | `/api/announcements/active` | 公开 | 启动器/官网拉取当前公开运维公告 |
| 4 | GET | `/api/economy/snapshot` | `economy.gameplay` | 获取角色完整经济快照 |
| 5 | GET | `/api/economy/announcements/active` | `economy.gameplay` | 游戏服拉取全服公告流 |

### 物品、仓库、装备与商店

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 6 | POST | `/api/economy/warehouse/deposit` | `economy.gameplay` | 背包物品/装备存入仓库 |
| 7 | POST | `/api/economy/warehouse/withdraw` | `economy.gameplay` | 仓库物品/装备取回背包 |
| 8 | POST | `/api/economy/equipment/equip` | `economy.gameplay` | 穿戴装备 |
| 9 | POST | `/api/economy/equipment/unequip` | `economy.gameplay` | 卸下装备 |
| 10 | POST | `/api/economy/equipment/repair` | `economy.gameplay` | 使用 AEB 修复装备耐久 |
| 11 | POST | `/api/economy/equipment/enhance` | `economy.gameplay` | 强化装备副词条实例 |
| 12 | POST | `/api/economy/equipment/npc-recycle` | `economy.gameplay` | 向 NPC 回收非 NFT 装备 |
| 13 | POST | `/api/economy/shop/buy` | `economy.gameplay` | 商店购买，金币即时交割，AEB 创建付款订单 |
| 14 | POST | `/api/economy/shop/sell` | `economy.gameplay` | 向商店出售背包物品或背包装备 |
| 15 | POST | `/api/economy/lottery/draw` | `economy.gameplay` | 创建抽奖付款订单并快照奖励计划 |

### 悬赏与 NFT

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 16 | GET | `/api/economy/bounty/board` | `economy.gameplay` | 返回并补齐角色悬赏板 |
| 17 | POST | `/api/economy/bounty/slots/unlock-gold` | `economy.gameplay` | 使用 Gold 解锁角色悬赏槽位 2 |
| 18 | POST | `/api/economy/bounty/slots/unlock-aeb` | `economy.gameplay` | 创建账号共享槽位 3-5 的 AEB 付款订单 |
| 19 | POST | `/api/economy/bounty/refresh` | `economy.gameplay` | 免费、Gold 或 Premium 刷新悬赏任务 |
| 20 | POST | `/api/economy/bounty/progress/combat` | `economy.gameplay` | 游戏服提交已结算副本的战斗进度 |
| 21 | POST | `/api/economy/bounty/submit-equipment` | `economy.gameplay` | 消耗合格背包装备完成稀有任务 |
| 22 | POST | `/api/economy/bounty/claim` | `economy.gameplay` | 领取已完成任务的徽章 |
| 23 | POST | `/api/economy/bounty/badges/draw` | `economy.gameplay` | 消耗悬赏徽章并发放奖励 |
| 24 | POST | `/api/economy/nft/mint/request` | `economy.gameplay` | 创建装备 NFT Mint 请求并扣费/锁定装备 |
| 25 | POST | `/api/economy/nft/mint/cancel` | `economy.gameplay` | 取消 Mint 请求并按原余额类别退款 |
| 26 | POST | `/api/economy/internal/nft/mint/confirm` | `economy.mint` | 确认 Mint 结果 |
| 27 | GET | `/api/economy/nft/assets` | `economy.gameplay` | 查询账号 NFT Asset |

### 副本、掉落、活动与 Boss

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 28 | POST | `/api/economy/dungeon/enter` | `economy.gameplay` | 创建绑定原游戏服的 Dungeon Run |
| 29 | POST | `/api/economy/dungeon/finish` | `economy.gameplay` | 原游戏服结束副本并结算 |
| 30 | POST | `/api/economy/loot/claim-player` | `economy.gameplay` | 领取一项 Loot Tray 奖励 |
| 31 | POST | `/api/economy/loot/claim-all` | `economy.gameplay` | 批量领取可放入背包的奖励 |
| 32 | POST | `/api/economy/loot/discard` | `economy.gameplay` | 丢弃 Loot Tray 奖励 |
| 33 | POST | `/api/economy/gathering/settle` | `economy.gameplay` | 结算采集节点 |
| 34 | POST | `/api/economy/farming/harvest` | `economy.gameplay` | 结算农作物收获 |
| 35 | POST | `/api/economy/boss/contribute` | `economy.gameplay` | 累加玩家 Boss 贡献 |
| 36 | POST | `/api/economy/boss/settle` | `economy.gameplay` | 结算玩家 Boss 奖励 |
| 37 | POST | `/api/economy/internal/boss/events/open` | `economy.boss_ops` | 开启 Boss Event |
| 38 | POST | `/api/economy/internal/boss/events/close` | `economy.boss_ops` | 关闭 Boss Event |
| 39 | POST | `/api/economy/internal/boss/events/settle` | `economy.boss_ops` | 标记 Boss Event 已结算 |
| 40 | GET | `/api/economy/internal/boss/events/active` | `economy.boss_ops` | 查询活动中的 Boss Event |

### 背包、成长购买与交易市场

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 41 | POST | `/api/economy/inventory/organize` | `economy.gameplay` | 整理背包 |
| 42 | POST | `/api/economy/warehouse/organize` | `economy.gameplay` | 整理仓库 |
| 43 | POST | `/api/economy/inventory/discard` | `economy.gameplay` | 丢弃背包物品/装备 |
| 44 | POST | `/api/economy/inventory/synthesize` | `economy.gameplay` | 按 Recipe 合成 |
| 45 | POST | `/api/economy/inventory/bag/expand` | `economy.gameplay` | 创建背包扩容付款订单 |
| 46 | POST | `/api/economy/license/purchase` | `economy.gameplay` | 创建交易许可证付款订单 |
| 47 | POST | `/api/economy/license/buy` | `economy.gameplay` | `/api/economy/license/purchase` 的别名 |
| 48 | GET | `/api/economy/marketplace/listings` | `economy.gameplay` | 查询市场挂单 |
| 49 | GET | `/api/economy/marketplace/listings/mine` | `economy.gameplay` | 查询自己的挂单 |
| 50 | GET | `/api/economy/marketplace/slots` | `economy.gameplay` | 查询挂单槽位 |
| 51 | POST | `/api/economy/marketplace/list` | `economy.gameplay` | 创建物品/装备挂单 |
| 52 | POST | `/api/economy/marketplace/listings/{listingId}/buy` | `economy.gameplay` | 购买挂单 |
| 53 | POST | `/api/economy/marketplace/listings/{listingId}/cancel` | `economy.gameplay` | 取消自己的挂单 |
| 54 | POST | `/api/economy/marketplace/slots/expand-material` | `economy.gameplay` | 使用材料扩展挂单槽位 |
| 55 | POST | `/api/economy/marketplace/slots/expand-wallet` | `economy.gameplay` | 创建链上付款扩槽订单 |
| 56 | POST | `/api/economy/marketplace/slots/expand-wallet/submit` | `economy.gameplay` | 玩家提交链上付款签名 |
| 57 | POST | `/api/economy/internal/payments/submit` | `economy.payments` | 支付操作器提交并验证付款签名 |

### AEB、支付确认与 Worker/Internal

| # | 方法 | 路径 | 认证 | 用途 |
| ---: | --- | --- | --- | --- |
| 58 | POST | `/api/economy/rewards/grant-locked` | `economy.rewards` | 发放带冷却期的 Locked AEB |
| 59 | POST | `/api/chain/token/claim` | `economy.gameplay` | 申请 AEB 提现 |
| 60 | GET | `/api/chain/token/ledger` | `economy.gameplay` | 查询账号经济账本 |
| 61 | POST | `/api/economy/internal/unlocks/settle` | `economy.worker` | 解锁到期 Locked AEB |
| 62 | POST | `/api/economy/internal/withdrawals/process` | `economy.worker` | 推进提现队列 |
| 63 | POST | `/api/economy/internal/chain/deposits/scan` | `economy.worker` | 扫描 Solana 充值 |
| 64 | POST | `/api/economy/internal/chain/payouts/submit` | `economy.worker` | 提交待处理 Solana Payout |
| 65 | POST | `/api/economy/internal/chain/payouts/confirm` | `economy.worker` | 确认 Solana Payout |
| 66 | POST | `/api/economy/internal/equipment/npc-recycle/purge` | `economy.worker` | 物理删除已过恢复期的 NPC 回收装备 |
| 67 | POST | `/api/economy/internal/payments/confirm` | `economy.payments` | 确认并履约已提交的 Payment Order |
