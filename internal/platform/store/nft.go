package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type NFTRules struct {
	MintFeeToken int64
	MinRarity    int
}

func (r NFTRules) WithDefaults() NFTRules {
	if r.MintFeeToken <= 0 {
		r.MintFeeToken = 200
	}
	if r.MinRarity <= 0 {
		r.MinRarity = 3
	}
	return r
}

type NFTAsset struct {
	ID              int64      `json:"id"`
	AccountID       int64      `json:"accountId"`
	SourceAssetType string     `json:"sourceAssetType"`
	SourceAssetID   int64      `json:"sourceAssetId"`
	MintAddress     string     `json:"mintAddress,omitempty"`
	MetadataURI     string     `json:"metadataUri,omitempty"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"createdAt"`
	MintedAt        *time.Time `json:"mintedAt,omitempty"`
	EquipmentUID    string     `json:"equipmentUid,omitempty"`
}

type NFTMintRequest struct {
	ID              int64      `json:"id"`
	AccountID       int64      `json:"accountId"`
	NFTAssetID      int64      `json:"nftAssetId,omitempty"`
	SourceAssetType string     `json:"sourceAssetType"`
	SourceAssetID   int64      `json:"sourceAssetId"`
	MintFeeToken    int64      `json:"mintFeeToken"`
	Status          string     `json:"status"`
	TxSignature     string     `json:"txSignature,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	SubmittedAt     *time.Time `json:"submittedAt,omitempty"`
	ConfirmedAt     *time.Time `json:"confirmedAt,omitempty"`
	EquipmentUID    string     `json:"equipmentUid,omitempty"`
}

type NFTMintRequestInput struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	EquipmentUID string
	Rules        NFTRules
}

type NFTMintRequestResult struct {
	Request  NFTMintRequest  `json:"request"`
	Asset    NFTAsset        `json:"asset"`
	Snapshot EconomySnapshot `json:"snapshot"`
}

type NFTMintConfirmInput struct {
	OpID        string
	RequestID   int64
	MintAddress string
	TxSignature string
	MetadataURI string
}

func (s *PostgresStore) RequestNFTMint(req NFTMintRequestInput) (NFTMintRequestResult, error) {
	rules := req.Rules.WithDefaults()
	uid := strings.TrimSpace(req.EquipmentUID)
	if uid == "" {
		return NFTMintRequestResult{}, errors.New("equipmentUid is required")
	}
	return runIdempotentAction(s, "nft_mint_request", req.OpID, req.AccountID, req.CharacterID, func(ctx context.Context, tx pgx.Tx) (NFTMintRequestResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return NFTMintRequestResult{}, err
		}
		var equipID int64
		var rarity int
		var location, bindType string
		err := tx.QueryRow(ctx, `
			SELECT id, rarity, location, bind_type
			FROM equipment_items
			WHERE equipment_uid = $1 AND account_id = $2 AND character_id = $3
			FOR UPDATE
		`, uid, req.AccountID, req.CharacterID).Scan(&equipID, &rarity, &location, &bindType)
		if errors.Is(err, pgx.ErrNoRows) {
			return NFTMintRequestResult{}, ErrNotFound
		}
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		if location != "IN_BAG" && location != "IN_WAREHOUSE" {
			return NFTMintRequestResult{}, fmt.Errorf("equipment location %s cannot be minted", location)
		}
		if rarity < rules.MinRarity {
			return NFTMintRequestResult{}, fmt.Errorf("equipment rarity %d below mint minimum %d", rarity, rules.MinRarity)
		}
		var existing int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM nft_assets
			WHERE source_asset_type = 'EQUIPMENT' AND source_asset_id = $1
				AND status IN ('OFFCHAIN', 'MINT_REQUESTED', 'MINTED')
		`, equipID).Scan(&existing); err != nil {
			return NFTMintRequestResult{}, err
		}
		if existing > 0 {
			return NFTMintRequestResult{}, errors.New("equipment already has an nft record")
		}

		spend, err := s.spendTokenInTx(ctx, tx, req.AccountID, rules.MintFeeToken)
		if err != nil {
			return NFTMintRequestResult{}, fmt.Errorf("mint fee: %w", err)
		}
		burn := bpsCeil(rules.MintFeeToken, 1000)
		recycle := bpsCeil(rules.MintFeeToken, 8000)
		rewards := rules.MintFeeToken - burn - recycle
		if rewards < 0 {
			rewards = 0
		}
		if err := s.insertSystemConsumption(ctx, tx, req.OpID, req.AccountID, req.CharacterID, spend, "NFT_MINT_FEE",
			rules.MintFeeToken, burn, recycle, rewards, fmt.Sprintf(`{"equipmentUid":%q}`, uid)); err != nil {
			return NFTMintRequestResult{}, err
		}

		var assetID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO nft_assets (account_id, source_asset_type, source_asset_id, status)
			VALUES ($1, 'EQUIPMENT', $2, 'MINT_REQUESTED')
			RETURNING id
		`, req.AccountID, equipID).Scan(&assetID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		var requestID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO nft_mint_requests (
				account_id, nft_asset_id, source_asset_type, source_asset_id, mint_fee_token, status, metadata
			) VALUES ($1, $2, 'EQUIPMENT', $3, $4, 'PAID', jsonb_build_object('equipmentUid', $5::text, 'opId', $6::text))
			RETURNING id
		`, req.AccountID, assetID, equipID, rules.MintFeeToken, uid, req.OpID).Scan(&requestID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET location = 'MINT_PENDING', slot = NULL, equip_slot = NULL, updated_at = NOW()
			WHERE id = $1
		`, equipID); err != nil {
			return NFTMintRequestResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "NFT_MINT_REQUESTED", "AEB", rules.MintFeeToken, req.OpID); err != nil {
			return NFTMintRequestResult{}, err
		}

		asset, err := s.loadNFTAsset(ctx, tx, assetID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		asset.EquipmentUID = uid
		request, err := s.loadNFTMintRequest(ctx, tx, requestID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		request.EquipmentUID = uid
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		return NFTMintRequestResult{Request: request, Asset: asset, Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) ConfirmNFTMint(req NFTMintConfirmInput) (NFTMintRequestResult, error) {
	if req.RequestID <= 0 {
		return NFTMintRequestResult{}, errors.New("requestId is required")
	}
	return runIdempotentAction(s, "nft_mint_confirm", req.OpID, 0, 0, func(ctx context.Context, tx pgx.Tx) (NFTMintRequestResult, error) {
		request, err := s.lockNFTMintRequest(ctx, tx, req.RequestID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		if request.Status == "CONFIRMED" {
			asset, err := s.loadNFTAsset(ctx, tx, request.NFTAssetID)
			if err != nil {
				return NFTMintRequestResult{}, err
			}
			return NFTMintRequestResult{Request: request, Asset: asset}, nil
		}
		if request.Status != "PAID" && request.Status != "SUBMITTED" {
			return NFTMintRequestResult{}, fmt.Errorf("request status %s cannot be confirmed", request.Status)
		}
		now := time.Now().UTC()
		mintAddr := strings.TrimSpace(req.MintAddress)
		if mintAddr == "" {
			mintAddr = fmt.Sprintf("nft_stub_%d_%s", request.ID, now.Format("20060102150405"))
		}
		txSig := strings.TrimSpace(req.TxSignature)
		if txSig == "" {
			txSig = "stub_mint_" + mintAddr
		}
		metaURI := strings.TrimSpace(req.MetadataURI)

		if _, err := tx.Exec(ctx, `
			UPDATE nft_mint_requests
			SET status = 'CONFIRMED', tx_signature = $2, submitted_at = COALESCE(submitted_at, $3), confirmed_at = $3
			WHERE id = $1
		`, request.ID, txSig, now); err != nil {
			return NFTMintRequestResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE nft_assets
			SET status = 'MINTED', mint_address = $2, metadata_uri = NULLIF($3, ''), minted_at = $4
			WHERE id = $1
		`, request.NFTAssetID, mintAddr, metaURI, now); err != nil {
			return NFTMintRequestResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET location = 'ON_CHAIN', minted_nft_id = $2, updated_at = NOW()
			WHERE id = $1 AND location = 'MINT_PENDING'
		`, request.SourceAssetID, request.NFTAssetID); err != nil {
			return NFTMintRequestResult{}, err
		}
		var characterID int64
		_ = tx.QueryRow(ctx, `SELECT COALESCE(character_id, 0) FROM equipment_items WHERE id = $1`, request.SourceAssetID).Scan(&characterID)
		if err := s.insertEconomyLedger(ctx, tx, request.AccountID, characterID, "NFT_MINT_CONFIRMED", mintAddr, request.MintFeeToken, req.OpID); err != nil {
			return NFTMintRequestResult{}, err
		}
		request, err = s.loadNFTMintRequest(ctx, tx, request.ID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		asset, err := s.loadNFTAsset(ctx, tx, request.NFTAssetID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		return NFTMintRequestResult{Request: request, Asset: asset}, nil
	})
}

func (s *PostgresStore) CancelNFTMint(opID string, accountID, requestID int64) (NFTMintRequestResult, error) {
	return runIdempotentAction(s, "nft_mint_cancel", opID, accountID, 0, func(ctx context.Context, tx pgx.Tx) (NFTMintRequestResult, error) {
		request, err := s.lockNFTMintRequest(ctx, tx, requestID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		if request.AccountID != accountID {
			return NFTMintRequestResult{}, ErrForbidden
		}
		if request.Status != "PAID" && request.Status != "REQUESTED" {
			return NFTMintRequestResult{}, fmt.Errorf("request status %s cannot be cancelled", request.Status)
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, `
			UPDATE nft_mint_requests SET status = 'CANCELLED' WHERE id = $1
		`, requestID); err != nil {
			return NFTMintRequestResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE nft_assets SET status = 'CANCELLED' WHERE id = $1
		`, request.NFTAssetID); err != nil {
			return NFTMintRequestResult{}, err
		}
		// Return equipment to bag.
		var characterID int64
		var loc string
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(character_id, 0), location FROM equipment_items WHERE id = $1 FOR UPDATE
		`, request.SourceAssetID).Scan(&characterID, &loc)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		if loc == "MINT_PENDING" && characterID > 0 {
			bagSlot, err := s.resolveStorageSlot(ctx, tx, characterID, "BAG", -1)
			if err != nil {
				return NFTMintRequestResult{}, err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'IN_BAG', slot = $2, updated_at = NOW()
				WHERE id = $1
			`, request.SourceAssetID, bagSlot); err != nil {
				return NFTMintRequestResult{}, err
			}
		}
		// Refund mint fee to withdrawable.
		if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
			return NFTMintRequestResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE account_tokens
			SET withdrawable_balance = withdrawable_balance + $2,
			    token_balance = token_balance + $2,
			    updated_at = NOW()
			WHERE account_id = $1
		`, accountID, request.MintFeeToken); err != nil {
			return NFTMintRequestResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, accountID, characterID, "NFT_MINT_CANCELLED", "AEB", request.MintFeeToken, opID); err != nil {
			return NFTMintRequestResult{}, err
		}
		_ = now
		request, err = s.loadNFTMintRequest(ctx, tx, requestID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		asset, err := s.loadNFTAsset(ctx, tx, request.NFTAssetID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		snap, err := s.economySnapshot(ctx, tx, accountID, characterID)
		if err != nil {
			return NFTMintRequestResult{}, err
		}
		return NFTMintRequestResult{Request: request, Asset: asset, Snapshot: snap}, nil
	})
}

func (s *PostgresStore) ListNFTAssets(accountID int64) ([]NFTAsset, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.account_id, a.source_asset_type, a.source_asset_id,
			COALESCE(a.mint_address, ''), COALESCE(a.metadata_uri, ''), a.status, a.created_at, a.minted_at,
			COALESCE(e.equipment_uid, '')
		FROM nft_assets a
		LEFT JOIN equipment_items e ON a.source_asset_type = 'EQUIPMENT' AND e.id = a.source_asset_id
		WHERE a.account_id = $1
		ORDER BY a.created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NFTAsset
	for rows.Next() {
		var row NFTAsset
		if err := rows.Scan(
			&row.ID, &row.AccountID, &row.SourceAssetType, &row.SourceAssetID,
			&row.MintAddress, &row.MetadataURI, &row.Status, &row.CreatedAt, &row.MintedAt, &row.EquipmentUID,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) lockNFTMintRequest(ctx context.Context, tx pgx.Tx, id int64) (NFTMintRequest, error) {
	var row NFTMintRequest
	var nftAssetID *int64
	var txSig *string
	var submitted, confirmed *time.Time
	err := tx.QueryRow(ctx, `
		SELECT id, account_id, nft_asset_id, source_asset_type, source_asset_id, mint_fee_token::bigint,
			status, tx_signature, created_at, submitted_at, confirmed_at
		FROM nft_mint_requests WHERE id = $1 FOR UPDATE
	`, id).Scan(
		&row.ID, &row.AccountID, &nftAssetID, &row.SourceAssetType, &row.SourceAssetID, &row.MintFeeToken,
		&row.Status, &txSig, &row.CreatedAt, &submitted, &confirmed,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return NFTMintRequest{}, ErrNotFound
	}
	if err != nil {
		return NFTMintRequest{}, err
	}
	if nftAssetID != nil {
		row.NFTAssetID = *nftAssetID
	}
	if txSig != nil {
		row.TxSignature = *txSig
	}
	row.SubmittedAt = submitted
	row.ConfirmedAt = confirmed
	return row, nil
}

func (s *PostgresStore) loadNFTMintRequest(ctx context.Context, q postgresReader, id int64) (NFTMintRequest, error) {
	var row NFTMintRequest
	var nftAssetID *int64
	var txSig *string
	var submitted, confirmed *time.Time
	err := q.QueryRow(ctx, `
		SELECT id, account_id, nft_asset_id, source_asset_type, source_asset_id, mint_fee_token::bigint,
			status, tx_signature, created_at, submitted_at, confirmed_at
		FROM nft_mint_requests WHERE id = $1
	`, id).Scan(
		&row.ID, &row.AccountID, &nftAssetID, &row.SourceAssetType, &row.SourceAssetID, &row.MintFeeToken,
		&row.Status, &txSig, &row.CreatedAt, &submitted, &confirmed,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return NFTMintRequest{}, ErrNotFound
	}
	if err != nil {
		return NFTMintRequest{}, err
	}
	if nftAssetID != nil {
		row.NFTAssetID = *nftAssetID
	}
	if txSig != nil {
		row.TxSignature = *txSig
	}
	row.SubmittedAt = submitted
	row.ConfirmedAt = confirmed
	return row, nil
}

func (s *PostgresStore) loadNFTAsset(ctx context.Context, q postgresReader, id int64) (NFTAsset, error) {
	var row NFTAsset
	var mint, uri *string
	err := q.QueryRow(ctx, `
		SELECT id, account_id, source_asset_type, source_asset_id, mint_address, metadata_uri, status, created_at, minted_at
		FROM nft_assets WHERE id = $1
	`, id).Scan(
		&row.ID, &row.AccountID, &row.SourceAssetType, &row.SourceAssetID, &mint, &uri, &row.Status, &row.CreatedAt, &row.MintedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return NFTAsset{}, ErrNotFound
	}
	if err != nil {
		return NFTAsset{}, err
	}
	if mint != nil {
		row.MintAddress = *mint
	}
	if uri != nil {
		row.MetadataURI = *uri
	}
	return row, nil
}
