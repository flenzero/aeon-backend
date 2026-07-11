package store

import (
	"context"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
)

type Repository interface {
	SaveWalletNonce(row WalletLoginNonce)
	WalletNonce(nonce, wallet string, now time.Time) (WalletLoginNonce, error)
	ConsumeWalletNonce(nonce, wallet string, now time.Time) error
	UpsertAccountByWallet(wallet string) Account
	Account(id int64) (Account, bool)
	SetBanned(accountID int64, banned bool) error
	SetAccountBan(accountID int64, banned bool, reason string) error
	AdminGetAccount(accountID int64, wallet string) (AdminAccountDetail, error)
	SetAccountRiskLevel(accountID int64, riskLevel int) error
	SetTradingLicense(accountID int64, granted bool) error
	CreateMarketRestriction(in CreateMarketRestrictionInput) (MarketRestriction, error)
	ListMarketRestrictions(accountID int64, activeOnly bool, limit, offset int) ([]MarketRestriction, error)
	RevokeMarketRestriction(id int64, adminID, reason string) (MarketRestriction, error)
	CreateRiskEvent(in CreateRiskEventInput) (RiskEvent, error)
	ListRiskEvents(accountID int64, limit, offset int) ([]RiskEvent, error)
	ListAudits(limit, offset int) ([]AuditEntry, error)
	AuditTarget(adminID, action, targetType, targetID, reason string) AuditEntry
	ListPaymentOrdersAdmin(filter AdminListFilter) ([]PaymentOrder, error)
	ListNFTMintRequests(filter AdminListFilter) ([]NFTMintRequest, error)
	GetHotWalletStatus(wallet string) (HotWalletStatus, error)
	SetHotWalletPayoutsPaused(wallet, network, tokenMint string, paused bool) (HotWalletStatus, error)
	AdminRevokeAccountSessions(accountID int64) (int64, error)
	CreateCharacter(accountID int64, name string) (Character, error)
	Characters(accountID int64) []Character
	Character(accountID, characterID int64) (Character, bool)
	EconomySnapshot(accountID, characterID int64) (EconomySnapshot, error)
	WarehouseDeposit(req EconomyActionRequest) (EconomySnapshot, error)
	WarehouseWithdraw(req EconomyActionRequest) (EconomySnapshot, error)
	EquipItem(req EconomyActionRequest) (EconomySnapshot, error)
	UnequipItem(req EconomyActionRequest) (EconomySnapshot, error)
	DungeonEnter(req DungeonEnterRequest) (DungeonResult, error)
	DungeonFinish(req DungeonFinishRequest) (DungeonResult, error)
	LootClaim(req LootActionRequest) (EconomySnapshot, error)
	LootClaimAll(req LootActionRequest) (EconomySnapshot, error)
	LootDiscard(req LootActionRequest) (EconomySnapshot, error)
	GatheringSettle(req ActivitySettlementRequest) (ActivitySettlementResult, error)
	FarmingHarvest(req ActivitySettlementRequest) (ActivitySettlementResult, error)
	BossContribute(req BossContributeRequest) (BossContributeResult, error)
	BossContribution(accountID, bossEventID int64) (contribution int64, bossKey string, err error)
	BossSettle(req BossSettleRequest) (BossSettleResult, error)
	BossOpenEvent(req BossOpenEventRequest) (BossEvent, error)
	BossCloseEvent(req BossCloseEventRequest) (BossEvent, error)
	BossMarkSettled(req BossMarkSettledRequest) (BossEvent, error)
	BossListActiveEvents() ([]BossEvent, error)
	InventoryOrganize(req EconomyActionRequest, bagSlots int) (EconomySnapshot, error)
	WarehouseOrganize(req EconomyActionRequest, warehouseSlots int) (EconomySnapshot, error)
	InventoryDiscard(req InventoryDiscardRequest) (EconomySnapshot, error)
	Synthesize(req SynthesizeRequest) (EconomySnapshot, error)
	EquipmentRepair(req EquipmentRepairRequest) (EquipmentRepairResult, error)
	RequestNFTMint(req NFTMintRequestInput) (NFTMintRequestResult, error)
	ConfirmNFTMint(req NFTMintConfirmInput) (NFTMintRequestResult, error)
	CancelNFTMint(opID string, accountID, requestID int64) (NFTMintRequestResult, error)
	ListNFTAssets(accountID int64) ([]NFTAsset, error)
	MarketplaceCreateListing(req MarketplaceListRequest) (MarketplaceListResult, error)
	MarketplaceBuy(req MarketplaceBuyRequest) (MarketplaceBuyResult, error)
	MarketplaceCancel(req MarketplaceCancelRequest) (MarketplaceCancelResult, error)
	MarketplaceExpandMaterialSlots(req MarketplaceExpandSlotsRequest) (MarketplaceExpandResult, error)
	MarketplaceExpandWalletSlots(req MarketplaceExpandWalletRequest) (MarketplaceExpandWalletResult, error)
	MarketplaceSubmitWalletExpandPayment(req MarketplaceSubmitPaymentRequest) (PaymentOrder, error)
	SubmitPaymentOrderVerified(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, req MarketplaceSubmitPaymentRequest) (PaymentOrder, error)
	CreateBagExpandPayment(req GrowthPaymentRequest) (GrowthPaymentResult, error)
	CreateTradingLicensePayment(req GrowthPaymentRequest) (GrowthPaymentResult, error)
	MarketplaceSlots(accountID int64, rules MarketplaceRules) (MarketplaceSlots, error)
	MarketplaceListListings(filter MarketplaceListFilter) ([]MarketplaceListing, error)
	MarketplaceMyListings(accountID int64, status string, limit, offset int) ([]MarketplaceListing, error)
	ScanAndCreditDeposits(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig) (DepositScanResult, error)
	ProcessAutoWithdrawalsWithChain(now time.Time, singleMax, userDailyMax, globalHourlyMax, globalDailyMax int64, limit int, cfg ChainScanConfig) []Withdrawal
	SubmitSolanaPayouts(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, limit int) (PayoutJobResult, error)
	ConfirmSolanaPayouts(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, limit int) (PayoutJobResult, error)
	ConfirmPaymentOrder(ctx context.Context, orderID, reason string) (PaymentOrder, error)
	SaveTicket(ticket GameTicket)
	ConsumeTicket(ticket, serverID string, now time.Time) (GameTicket, error)
	CreateAccountSession(req CreateSessionRequest) (AccountSession, error)
	GetAccountSession(sessionID string) (AccountSession, error)
	TouchAccountSession(sessionID string, now time.Time) error
	RevokeAccountSession(sessionID string, now time.Time) error
	LookupRefreshToken(rawToken string, now time.Time) (RefreshTokenRecord, error)
	RotateRefreshToken(oldRaw, newRaw, sessionID string, accountID int64, expiresAt, now time.Time) error
	UpsertGameServer(server GameServer) (GameServer, error)
	HeartbeatGameServer(serverID string, onlinePlayers int, now time.Time) (GameServer, error)
	ListGameServers(status string) ([]GameServer, error)
	EnterOnlineSession(row OnlineSession) (OnlineSession, error)
	TouchOnlineSession(accountID int64, connectionID string, now time.Time) (OnlineSession, error)
	LeaveOnlineSession(accountID int64, connectionID string) (OnlineSession, error)
	GetOnlineSession(accountID int64) (OnlineSession, error)
	ListOnlineByServer(serverID string) ([]OnlineSession, error)
	SweepStaleOnlineSessions(olderThan time.Time) (int64, error)
	Token(accountID int64) AccountToken
	GrantLocked(accountID, amount int64, source, ref string, unlockAt time.Time) (LockedGame, error)
	SettleUnlocks(now time.Time, limit int) []LockedGame
	CreateWithdrawal(accountID int64, amount int64, wallet string, manual bool) (Withdrawal, error)
	ProcessAutoWithdrawals(now time.Time, singleMax, userDailyMax, globalHourlyMax, globalDailyMax int64, limit int) []Withdrawal
	ListWithdrawals(status string) []Withdrawal
	ReviewWithdrawal(id int64, approve bool, adminID, reason string) (Withdrawal, error)
	Audit(adminID, action, target, reason string) AuditEntry
	Ledger(accountID int64) []LedgerEntry
}
