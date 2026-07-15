package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) DeleteCharacter(accountID, characterID int64) error {
	if accountID <= 0 || characterID <= 0 {
		return errors.New("accountId and characterId are required")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		UPDATE characters
		SET is_deleted = TRUE, updated_at = NOW()
		WHERE account_id = $1 AND id = $2 AND is_deleted = FALSE
	`, accountID, characterID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM character_states
		WHERE character_id = $1
	`, characterID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) PlayerProfile(accountID, characterID int64) (PlayerState, error) {
	var out PlayerState
	var mapID string
	var positionRaw, appearanceRaw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT
			c.account_id,
			c.id,
			COALESCE(st.map_id, ''),
			COALESCE(st.position, '{}'::jsonb),
			COALESCE(st.play_time_sec, 0),
			COALESCE(st.hunger, 100)::float8,
			c.level,
			c.exp,
			c.appearance,
			c.name,
			COALESCE(st.last_played_at, 'epoch'::timestamptz),
			st.last_played_at IS NOT NULL
		FROM characters c
		LEFT JOIN character_states st ON st.character_id = c.id
		WHERE c.account_id = $1 AND c.id = $2 AND c.is_deleted = FALSE
	`, accountID, characterID).Scan(
		&out.AccountID,
		&out.CharacterID,
		&mapID,
		&positionRaw,
		&out.PlayTimeSec,
		&out.Hunger,
		&out.Level,
		&out.Exp,
		&appearanceRaw,
		&out.CharacterName,
		&out.LastPlayed,
		&out.HasLastPlayed,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerState{}, ErrNotFound
	}
	if err != nil {
		return PlayerState{}, err
	}
	out.CurrentMap = mapID
	var position map[string]any
	if len(positionRaw) > 0 {
		if err := json.Unmarshal(positionRaw, &position); err != nil {
			return PlayerState{}, err
		}
	}
	out.PosX = floatFromMap(position, "x")
	out.PosY = floatFromMap(position, "y")
	if len(appearanceRaw) > 0 {
		if err := json.Unmarshal(appearanceRaw, &out.Appearance); err != nil {
			return PlayerState{}, err
		}
	}
	if out.Appearance == nil {
		out.Appearance = map[string]any{}
	}
	return out, nil
}

func (s *PostgresStore) SavePlayerState(req PlayerSaveRequest) error {
	if req.AccountID <= 0 || req.CharacterID <= 0 {
		return errors.New("accountId and characterId are required")
	}
	if req.PlayTimeSec < 0 {
		return errors.New("playTimeSec must be non-negative")
	}
	if req.Hunger < 0 {
		return errors.New("hunger must be non-negative")
	}
	position, err := json.Marshal(map[string]any{
		"x": req.PosX,
		"y": req.PosY,
	})
	if err != nil {
		return err
	}
	var savedCharacterID int64
	err = s.pool.QueryRow(context.Background(), `
		INSERT INTO character_states (
			character_id,
			map_id,
			position,
			play_time_sec,
			hunger,
			last_played_at,
			updated_at
		)
		SELECT
			c.id,
			NULLIF($3, ''),
			$4::jsonb,
			$5,
			$6,
			NOW(),
			NOW()
		FROM characters c
		WHERE c.account_id = $1 AND c.id = $2 AND c.is_deleted = FALSE
		ON CONFLICT (character_id) DO UPDATE
		SET map_id = EXCLUDED.map_id,
			position = EXCLUDED.position,
			play_time_sec = EXCLUDED.play_time_sec,
			hunger = EXCLUDED.hunger,
			last_played_at = EXCLUDED.last_played_at,
			updated_at = NOW()
		RETURNING character_id
	`, req.AccountID, req.CharacterID, strings.TrimSpace(req.CurrentMap), string(position), req.PlayTimeSec, req.Hunger).Scan(&savedCharacterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) ClearProgressLeaderboard(limit int) (ClearProgressLeaderboard, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, name, highest_cleared_chapter, highest_cleared_floor, highest_cleared_at, dungeon_clear_count
		FROM characters
		WHERE is_deleted = FALSE AND highest_cleared_floor > 0
		ORDER BY highest_cleared_floor DESC, highest_cleared_at ASC NULLS LAST, id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return ClearProgressLeaderboard{}, err
	}
	defer rows.Close()
	out := ClearProgressLeaderboard{
		Intro:     "永久通关进度榜按角色历史最高 floorId 排名，同层时更早达成者优先。",
		Items:     []ClearProgressLeaderboardItem{},
		UpdatedAt: time.Now().UTC(),
	}
	rank := 1
	for rows.Next() {
		var item ClearProgressLeaderboardItem
		if err := rows.Scan(&item.CharacterID, &item.CharacterName, &item.HighestChapterID, &item.HighestFloorID, &item.FirstReachedAt, &item.ClearCount); err != nil {
			return ClearProgressLeaderboard{}, err
		}
		item.Rank = rank
		rank++
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return ClearProgressLeaderboard{}, err
	}
	return out, nil
}

func (s *PostgresStore) WeeklyScoreLeaderboard(now time.Time, limit int) (WeeklyScoreLeaderboard, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	period := weeklyLeaderboardPeriod(now.UTC())
	rows, err := s.pool.Query(context.Background(), `
		SELECT c.id, c.name, COALESCE(SUM(NULLIF(split_part(d.dungeon_key, ':', 4), '')::int), 0)::bigint, COUNT(d.id)::bigint, MAX(d.finished_at)
		FROM dungeon_runs d
		JOIN characters c ON c.id = d.character_id
		WHERE c.is_deleted = FALSE
			AND d.status = 'FINISHED'
			AND d.finished_at >= $1
			AND d.finished_at < $2
		GROUP BY c.id, c.name
		HAVING COUNT(d.id) > 0
		ORDER BY COALESCE(SUM(NULLIF(split_part(d.dungeon_key, ':', 4), '')::int), 0) DESC, MAX(d.finished_at) ASC, c.id ASC
		LIMIT $3
	`, period.StartsAt, period.EndsAt, limit)
	if err != nil {
		return WeeklyScoreLeaderboard{}, err
	}
	defer rows.Close()
	out := WeeklyScoreLeaderboard{
		Intro:     "7 天游积分榜每次成功通关按 floorId 加分，同一楼层可重复累计。",
		Period:    period,
		Items:     []WeeklyScoreLeaderboardItem{},
		UpdatedAt: time.Now().UTC(),
	}
	rank := 1
	for rows.Next() {
		var item WeeklyScoreLeaderboardItem
		if err := rows.Scan(&item.CharacterID, &item.CharacterName, &item.Score, &item.ClearCount, &item.LastScoredAt); err != nil {
			return WeeklyScoreLeaderboard{}, err
		}
		item.Rank = rank
		rank++
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return WeeklyScoreLeaderboard{}, err
	}
	return out, nil
}

func weeklyLeaderboardPeriod(now time.Time) LeaderboardPeriod {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if now.Before(start) {
		return LeaderboardPeriod{
			PeriodID: "weekly-score-20260701-0",
			StartsAt: start,
			EndsAt:   start.AddDate(0, 0, 7),
		}
	}
	elapsed := int(now.Sub(start).Hours()) / (24 * 7)
	startsAt := start.AddDate(0, 0, elapsed*7)
	return LeaderboardPeriod{
		PeriodID: fmt.Sprintf("weekly-score-20260701-%d", elapsed),
		StartsAt: startsAt,
		EndsAt:   startsAt.AddDate(0, 0, 7),
	}
}

func floatFromMap(in map[string]any, key string) float64 {
	if in == nil {
		return 0
	}
	switch value := in[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}
