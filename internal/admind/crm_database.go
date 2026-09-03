package admind

import (
	"context"
	"database/sql"
	"path/filepath"
)

func (service *Service) openCRMDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openSQLiteDatabase(ctx, service.crmDatabasePath(), leaveTheCRMStoreAsFound)
}

func leaveTheCRMStoreAsFound(context.Context, *sql.DB) error {
	return nil
}

func (service *Service) crmDatabasePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "crm.sqlite")
}
