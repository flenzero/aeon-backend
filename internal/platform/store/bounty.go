package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// BountyTask is immutable apart from progress and status. A row is replaced,
// never repurposed, when a player refreshes its slot.
type BountyTask struct {
	ID               int64  `json:"id"`
	SlotIndex        int    `json:"slotIndex"`
	TemplateID       string `json:"templateId"`
	Type             string `json:"type"`
	Difficulty       string `json:"difficulty"`
	ItemID           string `json:"itemId,omitempty"`
	MinRarity        int    `json:"minRarity,omitempty"`
	RequiredQuantity int64  `json:"requiredQuantity"`
	ProgressQuantity int64  `json:"progressQuantity"`
	Status           string `json:"status"`
	RewardItemID     string `json:"rewardItemId"`
	RewardQuantity   int64  `json:"rewardQuantity"`
}

type BountyTaskPlan struct {
	TemplateID, Type, Difficulty, ItemID, RewardItemID string
	MinRarity                                          int
	RequiredQuantity, RewardQuantity                   int64
}
type BountyBoardRequest struct {
	AccountID, CharacterID int64
	Plans                  map[int]BountyTaskPlan
}
type BountySlot struct {
	SlotIndex int         `json:"slotIndex"`
	Unlocked  bool        `json:"unlocked"`
	Task      *BountyTask `json:"task,omitempty"`
}
type BountyBoard struct {
	Slots                  []BountySlot `json:"slots"`
	FreeRefreshAvailableAt time.Time    `json:"freeRefreshAvailableAt"`
}
type BountyGoldUnlockRequest struct {
	OpID                             string
	AccountID, CharacterID, GoldCost int64
	Plans                            map[int]BountyTaskPlan
}
type BountyPaymentRequest struct {
	OpID                   string
	AccountID, CharacterID int64
	Purpose                string
	SlotIndex              int
	Amount                 int64
	ReceiverWallet         string
	RefreshPlans           map[int]BountyTaskPlan
}
type BountyPaymentResult struct {
	Order PaymentOrder `json:"order"`
	Board *BountyBoard `json:"board,omitempty"`
}
type BountyRefreshRequest struct {
	OpID                   string
	AccountID, CharacterID int64
	Mode                   string
	GoldCost               int64
	CooldownSeconds        int64
	Plans                  map[int]BountyTaskPlan
}
type BountyEquipmentSubmitRequest struct {
	OpID                   string
	AccountID, CharacterID int64
	SlotIndex              int
	EquipmentUID           string
}
type BountyClaimRequest struct {
	OpID                   string
	AccountID, CharacterID int64
	SlotIndex              int
}
type BountyBadgeDrawRequest struct {
	OpID                     string
	AccountID, CharacterID   int64
	BadgeItemID              string
	RewardType, RewardItemID string
	Amount                   int64
}
type BountyBadgeDrawResult struct {
	RewardType string     `json:"rewardType"`
	ItemID     string     `json:"itemId,omitempty"`
	Amount     int64      `json:"amount"`
	UnlockAt   *time.Time `json:"unlockAt,omitempty"`
}
type BountyCombatProgressRequest struct {
	OpID                   string
	AccountID, CharacterID int64
	DungeonRunID, ServerID string
	KillCount              int64
}

type bountyPaymentPayload struct {
	SlotIndex    int                    `json:"slotIndex,omitempty"`
	RefreshPlans map[int]BountyTaskPlan `json:"refreshPlans,omitempty"`
}

func (s *PostgresStore) BountyBoard(req BountyBoardRequest) (BountyBoard, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BountyBoard{}, err
	}
	defer rollback(ctx, tx)
	if err = s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
		return BountyBoard{}, err
	}
	if err = s.ensureBountyTasks(ctx, tx, req.AccountID, req.CharacterID, req.Plans); err != nil {
		return BountyBoard{}, err
	}
	board, err := s.bountyBoard(ctx, tx, req.AccountID, req.CharacterID)
	if err != nil {
		return BountyBoard{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return BountyBoard{}, err
	}
	return board, nil
}

func (s *PostgresStore) UnlockBountyGoldSlot(req BountyGoldUnlockRequest) (BountyBoard, error) {
	if req.GoldCost <= 0 {
		return BountyBoard{}, errors.New("invalid bounty gold unlock cost")
	}
	return runIdempotentAction(s, "bounty_unlock_gold", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BountyBoard, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BountyBoard{}, err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bounty_character_slots WHERE character_id=$1 AND slot_index=2)`, req.CharacterID).Scan(&exists); err != nil {
			return BountyBoard{}, err
		}
		if exists {
			return BountyBoard{}, errors.New("bounty slot 2 already unlocked")
		}
		tag, err := tx.Exec(ctx, `UPDATE character_wallets SET gold=gold-$2, updated_at=NOW() WHERE character_id=$1 AND gold >= $2`, req.CharacterID, req.GoldCost)
		if err != nil {
			return BountyBoard{}, err
		}
		if tag.RowsAffected() == 0 {
			return BountyBoard{}, ErrInsufficientBalance
		}
		if _, err = tx.Exec(ctx, `INSERT INTO bounty_character_slots(character_id,slot_index) VALUES($1,2)`, req.CharacterID); err != nil {
			return BountyBoard{}, err
		}
		if err = s.ensureBountyTasks(ctx, tx, req.AccountID, req.CharacterID, req.Plans); err != nil {
			return BountyBoard{}, err
		}
		if err = s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "BOUNTY_SLOT_UNLOCKED", "2", req.GoldCost, req.OpID); err != nil {
			return BountyBoard{}, err
		}
		return s.bountyBoard(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) CreateBountyPayment(req BountyPaymentRequest) (BountyPaymentResult, error) {
	if (req.Purpose != PaymentPurposeBountySlotUnlock && req.Purpose != PaymentPurposeBountyPremiumRefresh) || req.Amount <= 0 || strings.TrimSpace(req.ReceiverWallet) == "" {
		return BountyPaymentResult{}, errors.New("invalid bounty payment request")
	}
	return runIdempotentAction(s, "bounty_payment_create", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BountyPaymentResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BountyPaymentResult{}, err
		}
		if req.Purpose == PaymentPurposeBountySlotUnlock {
			if req.SlotIndex < 3 || req.SlotIndex > 5 {
				return BountyPaymentResult{}, errors.New("bounty AEB slot must be 3..5")
			}
			var owned bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bounty_account_slot_unlocks WHERE account_id=$1 AND slot_index=$2)`, req.AccountID, req.SlotIndex).Scan(&owned); err != nil {
				return BountyPaymentResult{}, err
			}
			if owned {
				return BountyPaymentResult{}, errors.New("bounty slot already unlocked")
			}
		}
		payload, err := json.Marshal(bountyPaymentPayload{SlotIndex: req.SlotIndex, RefreshPlans: req.RefreshPlans})
		if err != nil {
			return BountyPaymentResult{}, err
		}
		order, err := s.insertPaymentOrder(ctx, tx, req.AccountID, req.CharacterID, req.Purpose, req.Amount, req.ReceiverWallet, time.Now().UTC().Add(10*time.Minute), req.OpID, string(payload))
		if err != nil {
			return BountyPaymentResult{}, err
		}
		return BountyPaymentResult{Order: order}, nil
	})
}

func (s *PostgresStore) RefreshBounty(req BountyRefreshRequest) (BountyBoard, error) {
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode != "free" && req.Mode != "gold" {
		return BountyBoard{}, errors.New("refresh mode must be free or gold")
	}
	return runIdempotentAction(s, "bounty_refresh_"+req.Mode, req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BountyBoard, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BountyBoard{}, err
		}
		if req.Mode == "free" {
			var available time.Time
			err := tx.QueryRow(ctx, `INSERT INTO bounty_refreshes(character_id) VALUES($1) ON CONFLICT(character_id) DO UPDATE SET updated_at=NOW() RETURNING free_refresh_available_at`, req.CharacterID).Scan(&available)
			if err != nil {
				return BountyBoard{}, err
			}
			if time.Now().UTC().Before(available) {
				return BountyBoard{}, errors.New("free bounty refresh is cooling down")
			}
			_, err = tx.Exec(ctx, `UPDATE bounty_refreshes SET free_refresh_available_at=NOW()+INTERVAL '1 second' * $2,updated_at=NOW() WHERE character_id=$1`, req.CharacterID, req.CooldownSeconds)
			if err != nil {
				return BountyBoard{}, err
			}
		} else {
			tag, err := tx.Exec(ctx, `UPDATE character_wallets SET gold=gold-$2,updated_at=NOW() WHERE character_id=$1 AND gold >= $2`, req.CharacterID, req.GoldCost)
			if err != nil {
				return BountyBoard{}, err
			}
			if tag.RowsAffected() == 0 {
				return BountyBoard{}, ErrInsufficientBalance
			}
		}
		if err := s.replaceBountyTasks(ctx, tx, req.AccountID, req.CharacterID, req.Plans); err != nil {
			return BountyBoard{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "BOUNTY_REFRESHED", req.Mode, req.GoldCost, req.OpID); err != nil {
			return BountyBoard{}, err
		}
		return s.bountyBoard(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) SubmitBountyEquipment(req BountyEquipmentSubmitRequest) (BountyTask, error) {
	return runIdempotentAction(s, "bounty_submit_equipment", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BountyTask, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BountyTask{}, err
		}
		task, err := s.lockBountyTask(ctx, tx, req.CharacterID, req.SlotIndex, "submit_equipment")
		if err != nil {
			return BountyTask{}, err
		}
		tag, err := tx.Exec(ctx, `UPDATE equipment_items e SET location='CONSUMED',slot=NULL,equip_slot=NULL,updated_at=NOW() WHERE e.account_id=$1 AND e.character_id=$2 AND e.equipment_uid=$3 AND e.location='IN_BAG' AND e.rarity >= $4 AND NOT EXISTS (SELECT 1 FROM nft_assets n WHERE n.source_asset_type='EQUIPMENT' AND n.source_asset_id=e.id AND n.status IN ('MINT_REQUESTED','MINTED')) AND NOT EXISTS (SELECT 1 FROM marketplace_listings m WHERE m.asset_type='EQUIPMENT' AND m.asset_id=e.id AND m.status='ACTIVE')`, req.AccountID, req.CharacterID, strings.TrimSpace(req.EquipmentUID), task.MinRarity)
		if err != nil {
			return BountyTask{}, err
		}
		if tag.RowsAffected() == 0 {
			return BountyTask{}, ErrForbidden
		}
		return s.completeBountyTask(ctx, tx, task)
	})
}
func (s *PostgresStore) ClaimBounty(req BountyClaimRequest) (BountyTask, error) {
	return runIdempotentAction(s, "bounty_claim", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BountyTask, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BountyTask{}, err
		}
		task, err := s.lockBountyTask(ctx, tx, req.CharacterID, req.SlotIndex, "")
		if err != nil {
			return BountyTask{}, err
		}
		if task.Status != "COMPLETED" {
			return BountyTask{}, errors.New("bounty task is not completed")
		}
		_, err = s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, fmt.Sprintf("bounty-%d", task.ID), "bounty", DungeonRewardPlan{Items: []DungeonRewardGrant{{RewardType: "item", ItemID: task.RewardItemID, Quantity: task.RewardQuantity}}})
		if err != nil {
			return BountyTask{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE bounty_tasks SET status='CLAIMED',claimed_at=NOW() WHERE id=$1`, task.ID); err != nil {
			return BountyTask{}, err
		}
		task.Status = "CLAIMED"
		return task, nil
	})
}
func (s *PostgresStore) DrawBountyBadge(req BountyBadgeDrawRequest) (BountyBadgeDrawResult, error) {
	if req.Amount <= 0 {
		return BountyBadgeDrawResult{}, errors.New("invalid bounty badge reward")
	}
	return runIdempotentAction(s, "bounty_badge_draw", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BountyBadgeDrawResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BountyBadgeDrawResult{}, err
		}
		if err := s.consumeBagItem(ctx, tx, req.CharacterID, req.BadgeItemID, 1); err != nil {
			return BountyBadgeDrawResult{}, err
		}
		out := BountyBadgeDrawResult{RewardType: req.RewardType, ItemID: req.RewardItemID, Amount: req.Amount}
		switch req.RewardType {
		case "gold":
			tag, err := tx.Exec(ctx, `UPDATE character_wallets SET gold=gold+$2,updated_at=NOW() WHERE character_id=$1`, req.CharacterID, req.Amount)
			if err != nil {
				return out, err
			}
			if tag.RowsAffected() == 0 {
				return out, errors.New("character wallet missing")
			}
		case "item":
			_, err := s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, "bounty-badge", "bounty_badge", DungeonRewardPlan{Items: []DungeonRewardGrant{{RewardType: "item", ItemID: req.RewardItemID, Quantity: req.Amount}}})
			if err != nil {
				return out, err
			}
		case "locked_aeb":
			unlock := time.Now().UTC().Add(74 * time.Hour)
			_, err := s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, "bounty-badge", "bounty_badge", DungeonRewardPlan{TokenReward: req.Amount})
			if err != nil {
				return out, err
			}
			out.UnlockAt = &unlock
		default:
			return out, errors.New("unsupported bounty badge reward")
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "BOUNTY_BADGE_DRAW", req.BadgeItemID, req.Amount, req.OpID); err != nil {
			return out, err
		}
		return out, nil
	})
}
func (s *PostgresStore) ProgressBountyCombat(req BountyCombatProgressRequest) ([]BountyTask, error) {
	if strings.TrimSpace(req.ServerID) == "" {
		return nil, errors.New("combat proof is incomplete")
	}
	return runIdempotentAction(s, "bounty_combat_progress", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) ([]BountyTask, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return nil, err
		}
		var status, origin string
		var resultBytes []byte
		err := tx.QueryRow(ctx, `SELECT status,origin_server_id,result FROM dungeon_runs WHERE id=$1::uuid AND account_id=$2 AND character_id=$3 FOR UPDATE`, req.DungeonRunID, req.AccountID, req.CharacterID).Scan(&status, &origin, &resultBytes)
		if err != nil {
			return nil, err
		}
		if status != "FINISHED" || origin != req.ServerID {
			return nil, ErrForbidden
		}
		var result struct {
			Kills []DungeonKill `json:"kills"`
		}
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			return nil, fmt.Errorf("decode dungeon combat proof: %w", err)
		}
		var killCount int64
		for _, kill := range result.Kills {
			if kill.Quantity > 0 {
				killCount += kill.Quantity
			}
		}
		if killCount == 0 {
			return nil, errors.New("completed dungeon has no verified kills")
		}
		_, err = tx.Exec(ctx, `INSERT INTO bounty_combat_submissions(dungeon_run_id,character_id,op_id,kill_count) VALUES($1::uuid,$2,$3,$4)`, req.DungeonRunID, req.CharacterID, req.OpID, killCount)
		if err != nil {
			return nil, err
		}
		return s.advanceBountyTasks(ctx, tx, req.CharacterID, "combat", "", killCount)
	})
}

func (s *PostgresStore) fulfillBountyPaymentTx(ctx context.Context, tx pgx.Tx, order PaymentOrder) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM economy_payment_orders WHERE id=$1::uuid`, order.ID).Scan(&raw); err != nil {
		return err
	}
	var payload bountyPaymentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	switch order.Purpose {
	case PaymentPurposeBountySlotUnlock:
		if payload.SlotIndex < 3 || payload.SlotIndex > 5 {
			return errors.New("bounty slot payment missing slot")
		}
		_, err := tx.Exec(ctx, `INSERT INTO bounty_account_slot_unlocks(account_id,slot_index,payment_order_id) VALUES($1,$2,$3::uuid) ON CONFLICT(account_id,slot_index) DO NOTHING`, order.AccountID, payload.SlotIndex, order.ID)
		return err
	case PaymentPurposeBountyPremiumRefresh:
		return s.replaceBountyTasks(ctx, tx, order.AccountID, order.CharacterID, payload.RefreshPlans)
	}
	return errors.New("unsupported bounty payment")
}

func (s *PostgresStore) ensureBountyTasks(ctx context.Context, tx pgx.Tx, accountID, characterID int64, plans map[int]BountyTaskPlan) error {
	unlocked, err := s.bountyUnlockedSlots(ctx, tx, accountID, characterID)
	if err != nil {
		return err
	}
	for _, slot := range unlocked {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bounty_tasks WHERE character_id=$1 AND slot_index=$2 AND status IN ('ACTIVE','COMPLETED'))`, characterID, slot).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if err = s.insertBountyTask(ctx, tx, accountID, characterID, slot, plans[slot]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *PostgresStore) replaceBountyTasks(ctx context.Context, tx pgx.Tx, accountID, characterID int64, plans map[int]BountyTaskPlan) error {
	if len(plans) == 0 {
		return errors.New("no replaceable bounty tasks")
	}
	slots := make([]int, 0, len(plans))
	for slot := range plans {
		slots = append(slots, slot)
	}
	sort.Ints(slots)
	for _, slot := range slots {
		tag, err := tx.Exec(ctx, `UPDATE bounty_tasks SET status='REPLACED',replaced_at=NOW() WHERE character_id=$1 AND slot_index=$2 AND status='ACTIVE' AND difficulty='normal'`, characterID, slot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("bounty task cannot be refreshed")
		}
		if err = s.insertBountyTask(ctx, tx, accountID, characterID, slot, plans[slot]); err != nil {
			return err
		}
	}
	return nil
}
func (s *PostgresStore) insertBountyTask(ctx context.Context, tx pgx.Tx, accountID, characterID int64, slot int, plan BountyTaskPlan) error {
	if plan.RequiredQuantity <= 0 || plan.RewardQuantity <= 0 || strings.TrimSpace(plan.Type) == "" {
		return errors.New("invalid bounty task plan")
	}
	_, err := tx.Exec(ctx, `INSERT INTO bounty_tasks(account_id,character_id,slot_index,template_id,task_type,difficulty,item_id,min_rarity,required_quantity,reward_item_id,reward_quantity) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,0),$9,$10,$11)`, accountID, characterID, slot, plan.TemplateID, plan.Type, plan.Difficulty, plan.ItemID, plan.MinRarity, plan.RequiredQuantity, plan.RewardItemID, plan.RewardQuantity)
	return err
}
func (s *PostgresStore) bountyUnlockedSlots(ctx context.Context, tx pgx.Tx, accountID, characterID int64) ([]int, error) {
	slots := []int{1}
	var slot2 bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bounty_character_slots WHERE character_id=$1 AND slot_index=2)`, characterID).Scan(&slot2); err != nil {
		return nil, err
	}
	if slot2 {
		slots = append(slots, 2)
	}
	rows, err := tx.Query(ctx, `SELECT slot_index FROM bounty_account_slot_unlocks WHERE account_id=$1 ORDER BY slot_index`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot int
		if err := rows.Scan(&slot); err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	return slots, rows.Err()
}
func (s *PostgresStore) bountyBoard(ctx context.Context, tx pgx.Tx, accountID, characterID int64) (BountyBoard, error) {
	unlocked, err := s.bountyUnlockedSlots(ctx, tx, accountID, characterID)
	if err != nil {
		return BountyBoard{}, err
	}
	set := map[int]bool{}
	for _, x := range unlocked {
		set[x] = true
	}
	board := BountyBoard{Slots: make([]BountySlot, 0, 5)}
	for slot := 1; slot <= 5; slot++ {
		row := BountySlot{SlotIndex: slot, Unlocked: set[slot]}
		if row.Unlocked {
			task, err := s.readBountyTask(ctx, tx, characterID, slot)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return BountyBoard{}, err
			}
			if err == nil {
				row.Task = &task
			}
		}
		board.Slots = append(board.Slots, row)
	}
	_ = tx.QueryRow(ctx, `SELECT free_refresh_available_at FROM bounty_refreshes WHERE character_id=$1`, characterID).Scan(&board.FreeRefreshAvailableAt)
	return board, nil
}
func (s *PostgresStore) readBountyTask(ctx context.Context, tx pgx.Tx, characterID int64, slot int) (BountyTask, error) {
	var task BountyTask
	err := tx.QueryRow(ctx, `SELECT id,slot_index,template_id,task_type,difficulty,COALESCE(item_id,''),COALESCE(min_rarity,0),required_quantity,progress_quantity,status,reward_item_id,reward_quantity FROM bounty_tasks WHERE character_id=$1 AND slot_index=$2 AND status IN ('ACTIVE','COMPLETED')`, characterID, slot).Scan(&task.ID, &task.SlotIndex, &task.TemplateID, &task.Type, &task.Difficulty, &task.ItemID, &task.MinRarity, &task.RequiredQuantity, &task.ProgressQuantity, &task.Status, &task.RewardItemID, &task.RewardQuantity)
	return task, err
}
func (s *PostgresStore) lockBountyTask(ctx context.Context, tx pgx.Tx, characterID int64, slot int, kind string) (BountyTask, error) {
	task, err := s.readBountyTask(ctx, tx, characterID, slot)
	if err != nil {
		return task, err
	}
	if kind != "" && task.Type != kind {
		return task, ErrForbidden
	}
	if task.Status != "ACTIVE" {
		return task, errors.New("bounty task is not active")
	}
	return task, nil
}
func (s *PostgresStore) completeBountyTask(ctx context.Context, tx pgx.Tx, task BountyTask) (BountyTask, error) {
	_, err := tx.Exec(ctx, `UPDATE bounty_tasks SET progress_quantity=required_quantity,status='COMPLETED',completed_at=NOW() WHERE id=$1`, task.ID)
	if err != nil {
		return task, err
	}
	task.ProgressQuantity = task.RequiredQuantity
	task.Status = "COMPLETED"
	return task, nil
}
func (s *PostgresStore) advanceBountyTasks(ctx context.Context, tx pgx.Tx, characterID int64, kind, itemID string, quantity int64) ([]BountyTask, error) {
	rows, err := tx.Query(ctx, `UPDATE bounty_tasks SET progress_quantity=LEAST(required_quantity,progress_quantity+$4),status=CASE WHEN progress_quantity+$4>=required_quantity THEN 'COMPLETED' ELSE 'ACTIVE' END,completed_at=CASE WHEN progress_quantity+$4>=required_quantity THEN NOW() ELSE completed_at END WHERE character_id=$1 AND status='ACTIVE' AND task_type=$2 AND (NULLIF($3,'') IS NULL OR item_id=$3) RETURNING id,slot_index,template_id,task_type,difficulty,COALESCE(item_id,''),COALESCE(min_rarity,0),required_quantity,progress_quantity,status,reward_item_id,reward_quantity`, characterID, kind, itemID, quantity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BountyTask{}
	for rows.Next() {
		var t BountyTask
		if err := rows.Scan(&t.ID, &t.SlotIndex, &t.TemplateID, &t.Type, &t.Difficulty, &t.ItemID, &t.MinRarity, &t.RequiredQuantity, &t.ProgressQuantity, &t.Status, &t.RewardItemID, &t.RewardQuantity); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *PostgresStore) advanceGatheringBounties(ctx context.Context, tx pgx.Tx, characterID int64, plan DungeonRewardPlan) ([]BountyTask, error) {
	out := []BountyTask{}
	for _, r := range plan.Items {
		if r.RewardType != "equipment" && r.Quantity > 0 {
			changed, err := s.advanceBountyTasks(ctx, tx, characterID, "gather", r.ItemID, r.Quantity)
			if err != nil {
				return nil, err
			}
			out = append(out, changed...)
		}
	}
	return out, nil
}
