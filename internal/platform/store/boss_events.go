package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) BossOpenEvent(req BossOpenEventRequest) (BossEvent, error) {
	bossKey := strings.TrimSpace(req.BossKey)
	if bossKey == "" {
		return BossEvent{}, errors.New("bossKey is required")
	}
	startsAt := req.StartsAt.UTC()
	endsAt := req.EndsAt.UTC()
	if startsAt.IsZero() {
		startsAt = time.Now().UTC()
	}
	if endsAt.IsZero() || !endsAt.After(startsAt) {
		return BossEvent{}, errors.New("endsAt must be after startsAt")
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return BossEvent{}, err
	}

	return runIdempotentAction(s, "boss_event_open", req.OpID, 0, 0, func(ctx context.Context, tx pgx.Tx) (BossEvent, error) {
		var openCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM boss_events
			WHERE boss_key = $1 AND status = 'OPEN'
		`, bossKey).Scan(&openCount); err != nil {
			return BossEvent{}, err
		}
		if openCount > 0 {
			return BossEvent{}, errors.New("an OPEN boss event already exists for this bossKey")
		}
		var event BossEvent
		err := tx.QueryRow(ctx, `
			INSERT INTO boss_events (boss_key, status, starts_at, ends_at, metadata)
			VALUES ($1, 'OPEN', $2, $3, $4)
			RETURNING id, boss_key, status, starts_at, ends_at, created_at
		`, bossKey, startsAt, endsAt, metadataBytes).Scan(
			&event.ID,
			&event.BossKey,
			&event.Status,
			&event.StartsAt,
			&event.EndsAt,
			&event.CreatedAt,
		)
		if err != nil {
			return BossEvent{}, err
		}
		return event, nil
	})
}

func (s *PostgresStore) BossCloseEvent(req BossCloseEventRequest) (BossEvent, error) {
	if req.BossEventID <= 0 {
		return BossEvent{}, errors.New("bossEventId is required")
	}
	return runIdempotentAction(s, "boss_event_close", req.OpID, 0, 0, func(ctx context.Context, tx pgx.Tx) (BossEvent, error) {
		bossKey, status, _, _, err := s.lockBossEvent(ctx, tx, req.BossEventID)
		if err != nil {
			return BossEvent{}, err
		}
		if status != "OPEN" {
			return BossEvent{}, errors.New("boss event is not open")
		}
		now := time.Now().UTC()
		var event BossEvent
		err = tx.QueryRow(ctx, `
			UPDATE boss_events
			SET status = 'SETTLING', ends_at = LEAST(ends_at, $2)
			WHERE id = $1
			RETURNING id, boss_key, status, starts_at, ends_at, created_at
		`, req.BossEventID, now).Scan(
			&event.ID,
			&event.BossKey,
			&event.Status,
			&event.StartsAt,
			&event.EndsAt,
			&event.CreatedAt,
		)
		if err != nil {
			return BossEvent{}, err
		}
		_ = bossKey
		return event, nil
	})
}

func (s *PostgresStore) BossMarkSettled(req BossMarkSettledRequest) (BossEvent, error) {
	if req.BossEventID <= 0 {
		return BossEvent{}, errors.New("bossEventId is required")
	}
	return runIdempotentAction(s, "boss_event_settle", req.OpID, 0, 0, func(ctx context.Context, tx pgx.Tx) (BossEvent, error) {
		_, status, _, _, err := s.lockBossEvent(ctx, tx, req.BossEventID)
		if err != nil {
			return BossEvent{}, err
		}
		if status != "SETTLING" && status != "OPEN" {
			return BossEvent{}, errors.New("boss event is not ready to mark settled")
		}
		var event BossEvent
		err = tx.QueryRow(ctx, `
			UPDATE boss_events
			SET status = 'SETTLED'
			WHERE id = $1
			RETURNING id, boss_key, status, starts_at, ends_at, created_at
		`, req.BossEventID).Scan(
			&event.ID,
			&event.BossKey,
			&event.Status,
			&event.StartsAt,
			&event.EndsAt,
			&event.CreatedAt,
		)
		if err != nil {
			return BossEvent{}, err
		}
		return event, nil
	})
}

func (s *PostgresStore) BossListActiveEvents() ([]BossEvent, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, boss_key, status, starts_at, ends_at, created_at
		FROM boss_events
		WHERE status IN ('OPEN', 'SETTLING')
		ORDER BY starts_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BossEvent{}
	for rows.Next() {
		var event BossEvent
		if err := rows.Scan(&event.ID, &event.BossKey, &event.Status, &event.StartsAt, &event.EndsAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
