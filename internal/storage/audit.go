package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// InsertAudit добавляет запись в журнал аудита.
// userID может быть 0/NULL для событий без известного пользователя (не наш случай).
// details сериализуется в JSONB; nil-details допустим.
func (s *Storage) InsertAudit(ctx context.Context, userID int64, action, entity string, details map[string]any) error {
	var payload []byte
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("marshal audit details: %w", err)
		}
		payload = b
	}
	const q = `INSERT INTO audit_log (user_id, action, entity, details) VALUES ($1, $2, $3, $4)`
	if _, err := s.pool.Exec(ctx, q, userID, action, entity, payload); err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}
