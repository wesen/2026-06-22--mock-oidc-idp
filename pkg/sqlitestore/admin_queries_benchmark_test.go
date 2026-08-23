package sqlitestore_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/sqlitestore"
)

func BenchmarkListAdminUsers10000(b *testing.B) {
	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, sqlitestore.DefaultConfig(filepath.Join(b.TempDir(), "idp.db")))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	transaction, err := store.SQLDB().BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	statement, err := transaction.PrepareContext(ctx, `
		INSERT INTO admin_user_projection
			(user_id, subject, login, email, display_name, disabled,
			 locked_until_ns, last_successful_login_at_ns,
			 active_session_count, active_grant_count,
			 created_at_ns, updated_at_ns, version)
		VALUES (?, ?, ?, ?, ?, 0, NULL, NULL, 0, 0, ?, ?, 1)`)
	if err != nil {
		b.Fatal(err)
	}
	now := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC).UnixNano()
	for index := range 10_000 {
		id := fmt.Sprintf("user-%05d", index)
		if _, err := statement.ExecContext(
			ctx, id, "subject-"+id, "login-"+id,
			id+"@example.test", "Person "+id, now, now+int64(index),
		); err != nil {
			b.Fatal(err)
		}
	}
	if err := statement.Close(); err != nil {
		b.Fatal(err)
	}
	if err := transaction.Commit(); err != nil {
		b.Fatal(err)
	}

	b.ReportMetric(10_000, "users")
	b.ResetTimer()
	for b.Loop() {
		rows, err := store.ListAdminUsers(
			ctx,
			idpadmin.UserFilter{Query: "user-099"},
			idpadmin.DefaultPageSize,
		)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) != idpadmin.DefaultPageSize {
			b.Fatalf("rows = %d, want %d", len(rows), idpadmin.DefaultPageSize)
		}
	}
}
