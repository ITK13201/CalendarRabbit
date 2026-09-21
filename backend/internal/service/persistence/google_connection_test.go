package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/google"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGoogleConnRepo(t *testing.T) *persistence.GoogleConnectionRepository {
	t.Helper()
	client := testsupport.NewClient(t)
	cipher, err := google.NewCipher("test-key")
	require.NoError(t, err)
	return persistence.NewGoogleConnectionRepository(client, cipher, nil)
}

func TestGoogleConnectionRepository_SaveGetClear(t *testing.T) {
	repo := newGoogleConnRepo(t)
	ctx := context.Background()

	// 未保存なら NotFound。
	_, err := repo.Get(ctx)
	assert.ErrorIs(t, err, derr.ErrNotFound)

	expiry := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	saved, err := repo.Save(ctx, persistence.GoogleConnectionInput{
		RefreshToken: "refresh-secret",
		AccessToken:  "access-token",
		TokenExpiry:  expiry,
		CalendarID:   "cal-123",
		Connected:    true,
	})
	require.NoError(t, err)
	assert.True(t, saved.Connected)

	// 取得時に復号されて平文が返る。
	got, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "refresh-secret", got.RefreshToken)
	assert.Equal(t, "cal-123", got.CalendarID)
	assert.True(t, expiry.Equal(got.TokenExpiry))

	// 更新（単一レコード運用）。
	_, err = repo.Save(ctx, persistence.GoogleConnectionInput{
		RefreshToken: "refresh-secret-2",
		CalendarID:   "cal-456",
		Connected:    true,
	})
	require.NoError(t, err)
	got, err = repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "refresh-secret-2", got.RefreshToken)
	assert.Equal(t, "cal-456", got.CalendarID)

	// Clear で破棄され NotFound に戻る。
	require.NoError(t, repo.Clear(ctx))
	_, err = repo.Get(ctx)
	assert.ErrorIs(t, err, derr.ErrNotFound)
}

// TestGoogleConnectionRepository_RefreshTokenStoredEncrypted は保存列が平文でないことを確認する。
func TestGoogleConnectionRepository_RefreshTokenStoredEncrypted(t *testing.T) {
	client := testsupport.NewClient(t)
	cipher, err := google.NewCipher("test-key")
	require.NoError(t, err)
	repo := persistence.NewGoogleConnectionRepository(client, cipher, nil)
	ctx := context.Background()

	_, err = repo.Save(ctx, persistence.GoogleConnectionInput{
		RefreshToken: "plaintext-refresh",
		Connected:    true,
	})
	require.NoError(t, err)

	// ent クライアントで生列を直接読み、平文が保存されていないことを確認。
	row, err := client.GoogleConnection.Query().First(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, "plaintext-refresh", row.RefreshToken)
	assert.NotEmpty(t, row.RefreshToken)

	// 誤った鍵では復号できない。
	wrongRepo := persistence.NewGoogleConnectionRepository(client, mustWrongCipher(t), nil)
	_, err = wrongRepo.Get(ctx)
	require.Error(t, err)
}

func mustWrongCipher(t *testing.T) *google.Cipher {
	t.Helper()
	c, err := google.NewCipher("a-different-key")
	require.NoError(t, err)
	return c
}
