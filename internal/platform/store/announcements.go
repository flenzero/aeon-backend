package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	AnnouncementKindRareReward = "RARE_REWARD"
	AnnouncementKindOpsNotice  = "OPS_NOTICE"

	AnnouncementStatusActive  = "ACTIVE"
	AnnouncementStatusRevoked = "REVOKED"

	AnnouncementDisplayPopup  = "POPUP"
	AnnouncementDisplayBanner = "BANNER"

	AnnouncementTemplateRareEquipment = "rare_equipment"
	AnnouncementTemplateRareMount     = "rare_mount"
	AnnouncementTemplateOpsNotice     = "ops_notice"
)

type AnnouncementTemplate struct {
	Code            string    `json:"code"`
	Kind            string    `json:"kind"`
	TitleTemplate   string    `json:"titleTemplate"`
	BodyTemplate    string    `json:"bodyTemplate"`
	DisplayMode     string    `json:"displayMode"`
	Priority        int       `json:"priority"`
	DurationSeconds int       `json:"durationSeconds"`
	Enabled         bool      `json:"enabled"`
	UpdatedBy       string    `json:"updatedBy,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type UpsertAnnouncementTemplateInput struct {
	Code            string
	Kind            string
	TitleTemplate   string
	BodyTemplate    string
	DisplayMode     string
	Priority        int
	DurationSeconds int
	Enabled         *bool
	UpdatedBy       string
	Reason          string
}

type Announcement struct {
	ID            int64          `json:"id"`
	Kind          string         `json:"kind"`
	Status        string         `json:"status"`
	TemplateCode  string         `json:"templateCode,omitempty"`
	DisplayMode   string         `json:"displayMode"`
	Title         string         `json:"title"`
	Body          string         `json:"body"`
	Priority      int            `json:"priority"`
	Scope         string         `json:"scope"`
	StartsAt      time.Time      `json:"startsAt"`
	EndsAt        *time.Time     `json:"endsAt,omitempty"`
	AccountID     int64          `json:"accountId,omitempty"`
	CharacterID   int64          `json:"characterId,omitempty"`
	CharacterName string         `json:"characterName,omitempty"`
	EventType     string         `json:"eventType,omitempty"`
	Source        string         `json:"source,omitempty"`
	RefType       string         `json:"refType,omitempty"`
	RefID         string         `json:"refId,omitempty"`
	ItemID        string         `json:"itemId,omitempty"`
	ItemName      string         `json:"itemName,omitempty"`
	EquipmentUID  string         `json:"equipmentUid,omitempty"`
	Rarity        int            `json:"rarity,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedBy     string         `json:"createdBy,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	RevokedBy     string         `json:"revokedBy,omitempty"`
	RevokedAt     *time.Time     `json:"revokedAt,omitempty"`
	RevokeReason  string         `json:"revokeReason,omitempty"`
}

type AnnouncementFilter struct {
	Kind   string
	Status string
	Limit  int
	Offset int
}

type CreateOpsAnnouncementInput struct {
	AdminID     string
	Title       string
	Body        string
	DisplayMode string
	Priority    int
	StartsAt    time.Time
	EndsAt      *time.Time
	Reason      string
}

type UpdateOpsAnnouncementInput struct {
	AdminID     string
	Title       string
	Body        string
	DisplayMode string
	Priority    int
	StartsAt    time.Time
	EndsAt      *time.Time
	Reason      string
}

type rareAnnouncementContext struct {
	AccountID      int64
	CharacterID    int64
	Source         string
	RefType        string
	RefID          string
	CreatedBy      string
	AnnouncementOn bool
}

func normalizeAnnouncementKind(kind string) (string, error) {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	switch kind {
	case "":
		return "", nil
	case AnnouncementKindRareReward, AnnouncementKindOpsNotice:
		return kind, nil
	default:
		return "", errors.New("kind must be RARE_REWARD or OPS_NOTICE")
	}
}

func normalizeAnnouncementStatus(status string) (string, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	switch status {
	case "":
		return "", nil
	case AnnouncementStatusActive, AnnouncementStatusRevoked:
		return status, nil
	default:
		return "", errors.New("status must be ACTIVE or REVOKED")
	}
}

func normalizeAnnouncementDisplayMode(mode string) (string, error) {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode == "" {
		return AnnouncementDisplayBanner, nil
	}
	switch mode {
	case AnnouncementDisplayPopup, AnnouncementDisplayBanner:
		return mode, nil
	default:
		return "", errors.New("displayMode must be POPUP or BANNER")
	}
}

func normalizeAnnouncementPriority(priority int) int {
	if priority <= 0 {
		return 50
	}
	if priority > 1000 {
		return 1000
	}
	return priority
}

func normalizeAnnouncementLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func validateAnnouncementText(title, body string) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return errors.New("title and body are required")
	}
	if utf8.RuneCountInString(title) > 120 {
		return errors.New("title must not exceed 120 characters")
	}
	if utf8.RuneCountInString(body) > 500 {
		return errors.New("body must not exceed 500 characters")
	}
	return nil
}

func renderAnnouncementTemplate(template string, values map[string]string) string {
	out := template
	for key, value := range values {
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}

func defaultAnnouncementTemplates() []AnnouncementTemplate {
	now := time.Now().UTC()
	return []AnnouncementTemplate{
		{
			Code: AnnouncementTemplateRareEquipment, Kind: AnnouncementKindRareReward,
			TitleTemplate: "超稀有装备出现", BodyTemplate: "恭喜 {characterName} 通过{source}获得 {rarity}星装备 {itemName}",
			DisplayMode: AnnouncementDisplayPopup, Priority: 900, DurationSeconds: 12, Enabled: true, CreatedAt: now, UpdatedAt: now,
		},
		{
			Code: AnnouncementTemplateRareMount, Kind: AnnouncementKindRareReward,
			TitleTemplate: "稀有坐骑出现", BodyTemplate: "恭喜 {characterName} 通过{source}获得稀有坐骑 {itemName}",
			DisplayMode: AnnouncementDisplayPopup, Priority: 950, DurationSeconds: 12, Enabled: true, CreatedAt: now, UpdatedAt: now,
		},
		{
			Code: AnnouncementTemplateOpsNotice, Kind: AnnouncementKindOpsNotice,
			TitleTemplate: "{title}", BodyTemplate: "{body}",
			DisplayMode: AnnouncementDisplayBanner, Priority: 500, DurationSeconds: 0, Enabled: true, CreatedAt: now, UpdatedAt: now,
		},
	}
}

func defaultAnnouncementTemplate(code string) (AnnouncementTemplate, bool) {
	for _, row := range defaultAnnouncementTemplates() {
		if row.Code == code {
			return row, true
		}
	}
	return AnnouncementTemplate{}, false
}

func (s *Store) ensureAnnouncementMapsLocked() {
	if s.announcementTemplates == nil {
		s.announcementTemplates = map[string]*AnnouncementTemplate{}
		for _, row := range defaultAnnouncementTemplates() {
			cp := row
			s.announcementTemplates[row.Code] = &cp
		}
	}
	if s.announcements == nil {
		s.announcements = map[int64]*Announcement{}
	}
}

func (s *Store) ListAnnouncementTemplates(kind string) ([]AnnouncementTemplate, error) {
	kind, err := normalizeAnnouncementKind(kind)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	out := make([]AnnouncementTemplate, 0, len(s.announcementTemplates))
	for _, row := range s.announcementTemplates {
		if kind == "" || row.Kind == kind {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Store) UpsertAnnouncementTemplate(in UpsertAnnouncementTemplateInput) (AnnouncementTemplate, error) {
	in.Code = strings.ToLower(strings.TrimSpace(in.Code))
	if in.Code == "" {
		return AnnouncementTemplate{}, errors.New("code is required")
	}
	kind, err := normalizeAnnouncementKind(in.Kind)
	if err != nil {
		return AnnouncementTemplate{}, err
	}
	if kind == "" {
		if existing, ok := defaultAnnouncementTemplate(in.Code); ok {
			kind = existing.Kind
		} else {
			return AnnouncementTemplate{}, errors.New("kind is required for custom templates")
		}
	}
	mode, err := normalizeAnnouncementDisplayMode(in.DisplayMode)
	if err != nil {
		return AnnouncementTemplate{}, err
	}
	if err := validateAnnouncementText(in.TitleTemplate, in.BodyTemplate); err != nil {
		return AnnouncementTemplate{}, err
	}
	if strings.TrimSpace(in.UpdatedBy) == "" || strings.TrimSpace(in.Reason) == "" {
		return AnnouncementTemplate{}, errors.New("updatedBy and reason are required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	duration := in.DurationSeconds
	if duration < 0 {
		return AnnouncementTemplate{}, errors.New("durationSeconds must be non-negative")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	if existing := s.announcementTemplates[in.Code]; existing != nil && in.Enabled == nil {
		enabled = existing.Enabled
	}
	row := AnnouncementTemplate{
		Code: in.Code, Kind: kind, TitleTemplate: strings.TrimSpace(in.TitleTemplate), BodyTemplate: strings.TrimSpace(in.BodyTemplate),
		DisplayMode: mode, Priority: normalizeAnnouncementPriority(in.Priority), DurationSeconds: duration, Enabled: enabled,
		UpdatedBy: strings.TrimSpace(in.UpdatedBy), CreatedAt: now, UpdatedAt: now,
	}
	if existing := s.announcementTemplates[in.Code]; existing != nil {
		row.CreatedAt = existing.CreatedAt
	}
	s.announcementTemplates[in.Code] = &row
	s.audits = append(s.audits, AuditEntry{ID: s.next(), AdminID: row.UpdatedBy, Action: "announcement_template_upsert", Target: "announcement_template:" + row.Code, Reason: in.Reason, CreatedAt: now})
	return row, nil
}

func (s *Store) ListAnnouncements(filter AnnouncementFilter) ([]Announcement, error) {
	kind, err := normalizeAnnouncementKind(filter.Kind)
	if err != nil {
		return nil, err
	}
	status, err := normalizeAnnouncementStatus(filter.Status)
	if err != nil {
		return nil, err
	}
	limit := normalizeAnnouncementLimit(filter.Limit)
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	rows := make([]Announcement, 0, len(s.announcements))
	for _, row := range s.announcements {
		if kind != "" && row.Kind != kind {
			continue
		}
		if status != "" && row.Status != status {
			continue
		}
		rows = append(rows, cloneAnnouncement(*row))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	if filter.Offset >= len(rows) {
		return []Announcement{}, nil
	}
	end := filter.Offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[filter.Offset:end], nil
}

func (s *Store) ListActiveAnnouncements(kind string, now time.Time, afterID int64, limit int) ([]Announcement, error) {
	kind, err := normalizeAnnouncementKind(kind)
	if err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit = normalizeAnnouncementLimit(limit)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	rows := make([]Announcement, 0, len(s.announcements))
	for _, row := range s.announcements {
		if kind != "" && row.Kind != kind {
			continue
		}
		if row.ID <= afterID || row.Status != AnnouncementStatusActive || row.StartsAt.After(now) {
			continue
		}
		if row.EndsAt != nil && !row.EndsAt.After(now) {
			continue
		}
		rows = append(rows, cloneAnnouncement(*row))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Priority == rows[j].Priority {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].Priority > rows[j].Priority
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func cloneAnnouncement(row Announcement) Announcement {
	if row.Metadata != nil {
		cp := make(map[string]any, len(row.Metadata))
		for key, value := range row.Metadata {
			cp[key] = value
		}
		row.Metadata = cp
	}
	return row
}

func (s *Store) CreateOpsAnnouncement(in CreateOpsAnnouncementInput) (Announcement, error) {
	mode, err := normalizeAnnouncementDisplayMode(in.DisplayMode)
	if err != nil {
		return Announcement{}, err
	}
	if err := validateAnnouncementText(in.Title, in.Body); err != nil {
		return Announcement{}, err
	}
	if strings.TrimSpace(in.AdminID) == "" || strings.TrimSpace(in.Reason) == "" {
		return Announcement{}, errors.New("adminId and reason are required")
	}
	starts := in.StartsAt
	if starts.IsZero() {
		starts = time.Now().UTC()
	}
	if in.EndsAt != nil && !in.EndsAt.After(starts) {
		return Announcement{}, errors.New("endsAt must be after startsAt")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	row := Announcement{
		ID: s.next(), Kind: AnnouncementKindOpsNotice, Status: AnnouncementStatusActive,
		TemplateCode: AnnouncementTemplateOpsNotice, DisplayMode: mode, Title: strings.TrimSpace(in.Title), Body: strings.TrimSpace(in.Body),
		Priority: normalizeAnnouncementPriority(in.Priority), Scope: "GLOBAL", StartsAt: starts, EndsAt: in.EndsAt,
		EventType: "OPS_NOTICE", CreatedBy: strings.TrimSpace(in.AdminID), CreatedAt: now,
	}
	s.announcements[row.ID] = &row
	s.audits = append(s.audits, AuditEntry{ID: s.next(), AdminID: row.CreatedBy, Action: "announcement_ops_create", Target: "announcement:" + strconv.FormatInt(row.ID, 10), Reason: in.Reason, CreatedAt: now})
	return row, nil
}

func (s *Store) UpdateOpsAnnouncement(id int64, in UpdateOpsAnnouncementInput) (Announcement, error) {
	if id <= 0 {
		return Announcement{}, errors.New("announcementId is invalid")
	}
	mode, err := normalizeAnnouncementDisplayMode(in.DisplayMode)
	if err != nil {
		return Announcement{}, err
	}
	if err := validateAnnouncementText(in.Title, in.Body); err != nil {
		return Announcement{}, err
	}
	if strings.TrimSpace(in.AdminID) == "" || strings.TrimSpace(in.Reason) == "" {
		return Announcement{}, errors.New("adminId and reason are required")
	}
	starts := in.StartsAt
	if starts.IsZero() {
		starts = time.Now().UTC()
	}
	if in.EndsAt != nil && !in.EndsAt.After(starts) {
		return Announcement{}, errors.New("endsAt must be after startsAt")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	row := s.announcements[id]
	if row == nil {
		return Announcement{}, ErrNotFound
	}
	if row.Kind != AnnouncementKindOpsNotice {
		return Announcement{}, ErrForbidden
	}
	row.Title = strings.TrimSpace(in.Title)
	row.Body = strings.TrimSpace(in.Body)
	row.DisplayMode = mode
	row.Priority = normalizeAnnouncementPriority(in.Priority)
	row.StartsAt = starts
	row.EndsAt = in.EndsAt
	s.audits = append(s.audits, AuditEntry{ID: s.next(), AdminID: strings.TrimSpace(in.AdminID), Action: "announcement_ops_update", Target: "announcement:" + strconv.FormatInt(row.ID, 10), Reason: in.Reason, CreatedAt: time.Now().UTC()})
	return cloneAnnouncement(*row), nil
}

func (s *Store) RevokeAnnouncement(id int64, adminID, reason string) (Announcement, error) {
	if id <= 0 {
		return Announcement{}, errors.New("announcementId is invalid")
	}
	adminID = strings.TrimSpace(adminID)
	reason = strings.TrimSpace(reason)
	if adminID == "" || reason == "" {
		return Announcement{}, errors.New("adminId and reason are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureAnnouncementMapsLocked()
	row := s.announcements[id]
	if row == nil {
		return Announcement{}, ErrNotFound
	}
	if row.Status != AnnouncementStatusRevoked {
		now := time.Now().UTC()
		row.Status = AnnouncementStatusRevoked
		row.RevokedBy = adminID
		row.RevokedAt = &now
		row.RevokeReason = reason
		s.audits = append(s.audits, AuditEntry{ID: s.next(), AdminID: adminID, Action: "announcement_revoke", Target: "announcement:" + strconv.FormatInt(row.ID, 10), Reason: reason, CreatedAt: now})
	}
	return cloneAnnouncement(*row), nil
}

func (s *PostgresStore) ListAnnouncementTemplates(kind string) ([]AnnouncementTemplate, error) {
	kind, err := normalizeAnnouncementKind(kind)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT code, kind, title_template, body_template, display_mode, priority, duration_seconds, enabled,
			COALESCE(updated_by, ''), created_at, updated_at
		FROM announcement_templates
		WHERE ($1 = '' OR kind = $1)
		ORDER BY code
	`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AnnouncementTemplate
	for rows.Next() {
		var row AnnouncementTemplate
		if err := rows.Scan(&row.Code, &row.Kind, &row.TitleTemplate, &row.BodyTemplate, &row.DisplayMode, &row.Priority, &row.DurationSeconds, &row.Enabled, &row.UpdatedBy, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpsertAnnouncementTemplate(in UpsertAnnouncementTemplateInput) (AnnouncementTemplate, error) {
	in.Code = strings.ToLower(strings.TrimSpace(in.Code))
	if in.Code == "" {
		return AnnouncementTemplate{}, errors.New("code is required")
	}
	kind, err := normalizeAnnouncementKind(in.Kind)
	if err != nil {
		return AnnouncementTemplate{}, err
	}
	if kind == "" {
		if existing, ok := defaultAnnouncementTemplate(in.Code); ok {
			kind = existing.Kind
		} else {
			return AnnouncementTemplate{}, errors.New("kind is required for custom templates")
		}
	}
	mode, err := normalizeAnnouncementDisplayMode(in.DisplayMode)
	if err != nil {
		return AnnouncementTemplate{}, err
	}
	if err := validateAnnouncementText(in.TitleTemplate, in.BodyTemplate); err != nil {
		return AnnouncementTemplate{}, err
	}
	if strings.TrimSpace(in.UpdatedBy) == "" || strings.TrimSpace(in.Reason) == "" {
		return AnnouncementTemplate{}, errors.New("updatedBy and reason are required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	duration := in.DurationSeconds
	if duration < 0 {
		return AnnouncementTemplate{}, errors.New("durationSeconds must be non-negative")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AnnouncementTemplate{}, err
	}
	defer rollback(ctx, tx)
	var row AnnouncementTemplate
	err = tx.QueryRow(ctx, `
		INSERT INTO announcement_templates (code, kind, title_template, body_template, display_mode, priority, duration_seconds, enabled, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (code) DO UPDATE SET
			kind = EXCLUDED.kind,
			title_template = EXCLUDED.title_template,
			body_template = EXCLUDED.body_template,
			display_mode = EXCLUDED.display_mode,
			priority = EXCLUDED.priority,
			duration_seconds = EXCLUDED.duration_seconds,
			enabled = CASE WHEN $10::boolean THEN EXCLUDED.enabled ELSE announcement_templates.enabled END,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
		RETURNING code, kind, title_template, body_template, display_mode, priority, duration_seconds, enabled, COALESCE(updated_by, ''), created_at, updated_at
	`, in.Code, kind, strings.TrimSpace(in.TitleTemplate), strings.TrimSpace(in.BodyTemplate), mode, normalizeAnnouncementPriority(in.Priority), duration, enabled, strings.TrimSpace(in.UpdatedBy), in.Enabled != nil).Scan(
		&row.Code, &row.Kind, &row.TitleTemplate, &row.BodyTemplate, &row.DisplayMode, &row.Priority, &row.DurationSeconds, &row.Enabled, &row.UpdatedBy, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return AnnouncementTemplate{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'announcement_template_upsert', 'announcement_template', $2, $3)
	`, in.UpdatedBy, row.Code, in.Reason); err != nil {
		return AnnouncementTemplate{}, err
	}
	return row, tx.Commit(ctx)
}

func (s *PostgresStore) ListAnnouncements(filter AnnouncementFilter) ([]Announcement, error) {
	kind, err := normalizeAnnouncementKind(filter.Kind)
	if err != nil {
		return nil, err
	}
	status, err := normalizeAnnouncementStatus(filter.Status)
	if err != nil {
		return nil, err
	}
	limit := normalizeAnnouncementLimit(filter.Limit)
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, kind, status, COALESCE(template_code, ''), display_mode, title, body, priority, scope,
			starts_at, ends_at, COALESCE(account_id, 0), COALESCE(character_id, 0), COALESCE(character_name, ''),
			COALESCE(event_type, ''), COALESCE(source, ''), COALESCE(ref_type, ''), COALESCE(ref_id, ''),
			COALESCE(item_id, ''), COALESCE(item_name, ''), COALESCE(equipment_uid, ''), COALESCE(rarity, 0),
			COALESCE(metadata, '{}'::jsonb), COALESCE(created_by, ''), created_at,
			COALESCE(revoked_by, ''), revoked_at, COALESCE(revoke_reason, '')
		FROM announcements
		WHERE ($1 = '' OR kind = $1) AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, kind, status, limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnouncements(rows)
}

func (s *PostgresStore) ListActiveAnnouncements(kind string, now time.Time, afterID int64, limit int) ([]Announcement, error) {
	kind, err := normalizeAnnouncementKind(kind)
	if err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit = normalizeAnnouncementLimit(limit)
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, kind, status, COALESCE(template_code, ''), display_mode, title, body, priority, scope,
			starts_at, ends_at, COALESCE(account_id, 0), COALESCE(character_id, 0), COALESCE(character_name, ''),
			COALESCE(event_type, ''), COALESCE(source, ''), COALESCE(ref_type, ''), COALESCE(ref_id, ''),
			COALESCE(item_id, ''), COALESCE(item_name, ''), COALESCE(equipment_uid, ''), COALESCE(rarity, 0),
			COALESCE(metadata, '{}'::jsonb), COALESCE(created_by, ''), created_at,
			COALESCE(revoked_by, ''), revoked_at, COALESCE(revoke_reason, '')
		FROM announcements
		WHERE id > $1
			AND ($2 = '' OR kind = $2)
			AND status = 'ACTIVE'
			AND starts_at <= $3
			AND (ends_at IS NULL OR ends_at > $3)
		ORDER BY priority DESC, id ASC
		LIMIT $4
	`, afterID, kind, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnouncements(rows)
}

func scanAnnouncements(rows pgx.Rows) ([]Announcement, error) {
	var out []Announcement
	for rows.Next() {
		var row Announcement
		var raw []byte
		if err := rows.Scan(
			&row.ID, &row.Kind, &row.Status, &row.TemplateCode, &row.DisplayMode, &row.Title, &row.Body, &row.Priority, &row.Scope,
			&row.StartsAt, &row.EndsAt, &row.AccountID, &row.CharacterID, &row.CharacterName,
			&row.EventType, &row.Source, &row.RefType, &row.RefID,
			&row.ItemID, &row.ItemName, &row.EquipmentUID, &row.Rarity,
			&raw, &row.CreatedBy, &row.CreatedAt, &row.RevokedBy, &row.RevokedAt, &row.RevokeReason,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &row.Metadata)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateOpsAnnouncement(in CreateOpsAnnouncementInput) (Announcement, error) {
	mode, err := normalizeAnnouncementDisplayMode(in.DisplayMode)
	if err != nil {
		return Announcement{}, err
	}
	if err := validateAnnouncementText(in.Title, in.Body); err != nil {
		return Announcement{}, err
	}
	if strings.TrimSpace(in.AdminID) == "" || strings.TrimSpace(in.Reason) == "" {
		return Announcement{}, errors.New("adminId and reason are required")
	}
	starts := in.StartsAt
	if starts.IsZero() {
		starts = time.Now().UTC()
	}
	if in.EndsAt != nil && !in.EndsAt.After(starts) {
		return Announcement{}, errors.New("endsAt must be after startsAt")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Announcement{}, err
	}
	defer rollback(ctx, tx)
	row, err := s.insertAnnouncementTx(ctx, tx, Announcement{
		Kind: AnnouncementKindOpsNotice, Status: AnnouncementStatusActive, TemplateCode: AnnouncementTemplateOpsNotice,
		DisplayMode: mode, Title: strings.TrimSpace(in.Title), Body: strings.TrimSpace(in.Body),
		Priority: normalizeAnnouncementPriority(in.Priority), Scope: "GLOBAL", StartsAt: starts, EndsAt: in.EndsAt,
		EventType: "OPS_NOTICE", CreatedBy: strings.TrimSpace(in.AdminID), Metadata: map[string]any{"reason": strings.TrimSpace(in.Reason)},
	}, "")
	if err != nil {
		return Announcement{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'announcement_ops_create', 'announcement', $2, $3)
	`, in.AdminID, strconv.FormatInt(row.ID, 10), in.Reason); err != nil {
		return Announcement{}, err
	}
	return row, tx.Commit(ctx)
}

func (s *PostgresStore) UpdateOpsAnnouncement(id int64, in UpdateOpsAnnouncementInput) (Announcement, error) {
	if id <= 0 {
		return Announcement{}, errors.New("announcementId is invalid")
	}
	mode, err := normalizeAnnouncementDisplayMode(in.DisplayMode)
	if err != nil {
		return Announcement{}, err
	}
	if err := validateAnnouncementText(in.Title, in.Body); err != nil {
		return Announcement{}, err
	}
	if strings.TrimSpace(in.AdminID) == "" || strings.TrimSpace(in.Reason) == "" {
		return Announcement{}, errors.New("adminId and reason are required")
	}
	starts := in.StartsAt
	if starts.IsZero() {
		starts = time.Now().UTC()
	}
	if in.EndsAt != nil && !in.EndsAt.After(starts) {
		return Announcement{}, errors.New("endsAt must be after startsAt")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Announcement{}, err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		UPDATE announcements
		SET title = $2, body = $3, display_mode = $4, priority = $5, starts_at = $6, ends_at = $7
		WHERE id = $1 AND kind = 'OPS_NOTICE'
	`, id, strings.TrimSpace(in.Title), strings.TrimSpace(in.Body), mode, normalizeAnnouncementPriority(in.Priority), starts, in.EndsAt)
	if err != nil {
		return Announcement{}, err
	}
	if tag.RowsAffected() == 0 {
		return Announcement{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'announcement_ops_update', 'announcement', $2, $3)
	`, in.AdminID, strconv.FormatInt(id, 10), in.Reason); err != nil {
		return Announcement{}, err
	}
	row, err := s.announcementByIDTx(ctx, tx, id)
	if err != nil {
		return Announcement{}, err
	}
	return row, tx.Commit(ctx)
}

func (s *PostgresStore) RevokeAnnouncement(id int64, adminID, reason string) (Announcement, error) {
	if id <= 0 {
		return Announcement{}, errors.New("announcementId is invalid")
	}
	adminID = strings.TrimSpace(adminID)
	reason = strings.TrimSpace(reason)
	if adminID == "" || reason == "" {
		return Announcement{}, errors.New("adminId and reason are required")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Announcement{}, err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		UPDATE announcements
		SET status = 'REVOKED', revoked_by = $2, revoked_at = COALESCE(revoked_at, NOW()), revoke_reason = COALESCE(NULLIF(revoke_reason, ''), $3)
		WHERE id = $1
	`, id, adminID, reason)
	if err != nil {
		return Announcement{}, err
	}
	if tag.RowsAffected() == 0 {
		return Announcement{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'announcement_revoke', 'announcement', $2, $3)
	`, adminID, strconv.FormatInt(id, 10), reason); err != nil {
		return Announcement{}, err
	}
	row, err := s.announcementByIDTx(ctx, tx, id)
	if err != nil {
		return Announcement{}, err
	}
	return row, tx.Commit(ctx)
}

func (s *PostgresStore) announcementByIDTx(ctx context.Context, tx pgx.Tx, id int64) (Announcement, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, kind, status, COALESCE(template_code, ''), display_mode, title, body, priority, scope,
			starts_at, ends_at, COALESCE(account_id, 0), COALESCE(character_id, 0), COALESCE(character_name, ''),
			COALESCE(event_type, ''), COALESCE(source, ''), COALESCE(ref_type, ''), COALESCE(ref_id, ''),
			COALESCE(item_id, ''), COALESCE(item_name, ''), COALESCE(equipment_uid, ''), COALESCE(rarity, 0),
			COALESCE(metadata, '{}'::jsonb), COALESCE(created_by, ''), created_at,
			COALESCE(revoked_by, ''), revoked_at, COALESCE(revoke_reason, '')
		FROM announcements
		WHERE id = $1
	`, id)
	if err != nil {
		return Announcement{}, err
	}
	defer rows.Close()
	found, err := scanAnnouncements(rows)
	if err != nil {
		return Announcement{}, err
	}
	if len(found) == 0 {
		return Announcement{}, ErrNotFound
	}
	return found[0], nil
}

func (s *PostgresStore) publishRareRewardAnnouncementsTx(ctx context.Context, tx pgx.Tx, plan DungeonRewardPlan, ann rareAnnouncementContext) ([]Announcement, error) {
	if !ann.AnnouncementOn {
		return nil, nil
	}
	source := strings.TrimSpace(ann.Source)
	if source == "" {
		return nil, errors.New("announcement source is required")
	}
	characterName, err := s.characterNameTx(ctx, tx, ann.CharacterID)
	if err != nil {
		return nil, err
	}
	out := []Announcement{}
	for _, reward := range plan.Items {
		if strings.ToLower(strings.TrimSpace(reward.RewardType)) != "equipment" {
			continue
		}
		category := strings.ToLower(strings.TrimSpace(reward.Category))
		isMount := category == "mount"
		if !isMount && reward.Rarity < 5 {
			continue
		}
		code := AnnouncementTemplateRareEquipment
		eventType := "RARE_EQUIPMENT"
		if isMount {
			code = AnnouncementTemplateRareMount
			eventType = "RARE_MOUNT"
		}
		template, ok, err := s.announcementTemplateTx(ctx, tx, code)
		if err != nil {
			return nil, err
		}
		if !ok || !template.Enabled {
			continue
		}
		itemName, err := s.itemNameTx(ctx, tx, reward.ItemID)
		if err != nil {
			return nil, err
		}
		equipmentUID := strings.TrimSpace(reward.EquipmentUID)
		values := map[string]string{
			"characterName": characterName,
			"source":        source,
			"itemName":      itemName,
			"itemId":        strings.TrimSpace(reward.ItemID),
			"rarity":        strconv.Itoa(reward.Rarity),
			"equipmentUid":  equipmentUID,
		}
		now := time.Now().UTC()
		var endsAt *time.Time
		if template.DurationSeconds > 0 {
			t := now.Add(time.Duration(template.DurationSeconds) * time.Second)
			endsAt = &t
		}
		dedupe := strings.Join([]string{"rare", eventType, ann.RefType, ann.RefID, firstNonEmpty(equipmentUID, reward.ItemID)}, ":")
		row, err := s.insertAnnouncementTx(ctx, tx, Announcement{
			Kind: AnnouncementKindRareReward, Status: AnnouncementStatusActive, TemplateCode: code, DisplayMode: template.DisplayMode,
			Title: renderAnnouncementTemplate(template.TitleTemplate, values), Body: renderAnnouncementTemplate(template.BodyTemplate, values),
			Priority: template.Priority, Scope: "GLOBAL", StartsAt: now, EndsAt: endsAt, AccountID: ann.AccountID, CharacterID: ann.CharacterID,
			CharacterName: characterName, EventType: eventType, Source: source, RefType: ann.RefType, RefID: ann.RefID,
			ItemID: strings.TrimSpace(reward.ItemID), ItemName: itemName, EquipmentUID: equipmentUID, Rarity: reward.Rarity,
			Metadata: map[string]any{"category": category, "quantity": reward.Quantity}, CreatedBy: ann.CreatedBy,
		}, dedupe)
		if err != nil {
			return nil, err
		}
		if row.ID != 0 {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *PostgresStore) announcementTemplateTx(ctx context.Context, tx pgx.Tx, code string) (AnnouncementTemplate, bool, error) {
	var row AnnouncementTemplate
	err := tx.QueryRow(ctx, `
		SELECT code, kind, title_template, body_template, display_mode, priority, duration_seconds, enabled,
			COALESCE(updated_by, ''), created_at, updated_at
		FROM announcement_templates
		WHERE code = $1
	`, strings.ToLower(strings.TrimSpace(code))).Scan(
		&row.Code, &row.Kind, &row.TitleTemplate, &row.BodyTemplate, &row.DisplayMode, &row.Priority, &row.DurationSeconds, &row.Enabled, &row.UpdatedBy, &row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AnnouncementTemplate{}, false, nil
	}
	if err != nil {
		return AnnouncementTemplate{}, false, err
	}
	return row, true, nil
}

func (s *PostgresStore) characterNameTx(ctx context.Context, tx pgx.Tx, characterID int64) (string, error) {
	var name string
	err := tx.QueryRow(ctx, `SELECT name FROM characters WHERE id = $1 AND is_deleted = FALSE`, characterID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

func (s *PostgresStore) itemNameTx(ctx context.Context, tx pgx.Tx, itemID string) (string, error) {
	itemID = strings.TrimSpace(itemID)
	var name string
	err := tx.QueryRow(ctx, `SELECT COALESCE(name, item_id) FROM item_catalog WHERE item_id = $1`, itemID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) || strings.TrimSpace(name) == "" {
		return itemID, nil
	}
	return name, err
}

func (s *PostgresStore) insertAnnouncementTx(ctx context.Context, tx pgx.Tx, row Announcement, dedupeKey string) (Announcement, error) {
	raw, err := json.Marshal(row.Metadata)
	if err != nil {
		return Announcement{}, err
	}
	if row.Status == "" {
		row.Status = AnnouncementStatusActive
	}
	if row.Scope == "" {
		row.Scope = "GLOBAL"
	}
	if row.StartsAt.IsZero() {
		row.StartsAt = time.Now().UTC()
	}
	var out Announcement
	var metadata []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO announcements (
			kind, status, template_code, display_mode, title, body, priority, scope, starts_at, ends_at,
			account_id, character_id, character_name, event_type, source, ref_type, ref_id,
			item_id, item_name, equipment_uid, rarity, metadata, created_by, dedupe_key
		)
		VALUES (
			$1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10,
			NULLIF($11, 0), NULLIF($12, 0), NULLIF($13, ''), NULLIF($14, ''), NULLIF($15, ''), NULLIF($16, ''), NULLIF($17, ''),
			NULLIF($18, ''), NULLIF($19, ''), NULLIF($20, ''), NULLIF($21, 0), $22, NULLIF($23, ''), NULLIF($24, '')
		)
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
		RETURNING id, kind, status, COALESCE(template_code, ''), display_mode, title, body, priority, scope,
			starts_at, ends_at, COALESCE(account_id, 0), COALESCE(character_id, 0), COALESCE(character_name, ''),
			COALESCE(event_type, ''), COALESCE(source, ''), COALESCE(ref_type, ''), COALESCE(ref_id, ''),
			COALESCE(item_id, ''), COALESCE(item_name, ''), COALESCE(equipment_uid, ''), COALESCE(rarity, 0),
			COALESCE(metadata, '{}'::jsonb), COALESCE(created_by, ''), created_at,
			COALESCE(revoked_by, ''), revoked_at, COALESCE(revoke_reason, '')
	`, row.Kind, row.Status, row.TemplateCode, row.DisplayMode, row.Title, row.Body, row.Priority, row.Scope, row.StartsAt, row.EndsAt,
		row.AccountID, row.CharacterID, row.CharacterName, row.EventType, row.Source, row.RefType, row.RefID,
		row.ItemID, row.ItemName, row.EquipmentUID, row.Rarity, raw, row.CreatedBy, strings.TrimSpace(dedupeKey),
	).Scan(
		&out.ID, &out.Kind, &out.Status, &out.TemplateCode, &out.DisplayMode, &out.Title, &out.Body, &out.Priority, &out.Scope,
		&out.StartsAt, &out.EndsAt, &out.AccountID, &out.CharacterID, &out.CharacterName,
		&out.EventType, &out.Source, &out.RefType, &out.RefID,
		&out.ItemID, &out.ItemName, &out.EquipmentUID, &out.Rarity,
		&metadata, &out.CreatedBy, &out.CreatedAt, &out.RevokedBy, &out.RevokedAt, &out.RevokeReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Announcement{}, nil
	}
	if err != nil {
		return Announcement{}, err
	}
	_ = json.Unmarshal(metadata, &out.Metadata)
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
