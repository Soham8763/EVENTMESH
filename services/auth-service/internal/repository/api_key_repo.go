package repository

import "database/sql"

type APIKeyRepository struct {
	db *sql.DB
}

func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) GetTenantID(apiKey string) (string, error) {
	var tenantID string

	err := r.db.QueryRow(`
		SELECT tenant_id
		FROM api_keys
		WHERE api_key = $1 AND is_active = TRUE
	`, apiKey).Scan(&tenantID)

	if err != nil {
		return "", err
	}

	return tenantID, nil
}
