package settings_test

import (
	"context"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo は settings.Repository のインメモリ実装。
type fakeRepo struct {
	current *entity.AppSetting
}

func (f *fakeRepo) Get(_ context.Context) (*entity.AppSetting, error) {
	if f.current == nil {
		return nil, derr.ErrNotFound
	}
	return f.current, nil
}

func (f *fakeRepo) Upsert(_ context.Context, in persistence.AppSettingInput) (*entity.AppSetting, error) {
	f.current = &entity.AppSetting{ID: 1, Timezone: in.Timezone, LLMProvider: in.LLMProvider, UpdatedAt: time.Now().UTC()}
	return f.current, nil
}

func TestGet_DefaultWhenUninitialized(t *testing.T) {
	uc := settings.New(&fakeRepo{}, nil)
	s, err := uc.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, settings.DefaultTimezone, s.Timezone)
	assert.Equal(t, settings.DefaultLLMProvider, s.LLMProvider)
}

func TestUpdate_ValidTimezone(t *testing.T) {
	repo := &fakeRepo{}
	uc := settings.New(repo, nil)

	updated, err := uc.Update(context.Background(), "UTC", settings.ProviderClaude)
	require.NoError(t, err)
	assert.Equal(t, "UTC", updated.Timezone)
	assert.Equal(t, settings.ProviderClaude, updated.LLMProvider)

	// 後続の Get に反映される
	got, err := uc.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "UTC", got.Timezone)
	assert.Equal(t, settings.ProviderClaude, got.LLMProvider)
}

func TestUpdate_DefaultsProviderWhenEmpty(t *testing.T) {
	repo := &fakeRepo{}
	uc := settings.New(repo, nil)

	updated, err := uc.Update(context.Background(), "UTC", "")
	require.NoError(t, err)
	assert.Equal(t, settings.DefaultLLMProvider, updated.LLMProvider)
}

func TestUpdate_InvalidProvider(t *testing.T) {
	repo := &fakeRepo{}
	uc := settings.New(repo, nil)

	_, err := uc.Update(context.Background(), "UTC", "gemini")
	require.Error(t, err)
	assert.True(t, derr.IsValidation(err))
	assert.Nil(t, repo.current)
}

func TestUpdate_InvalidTimezone(t *testing.T) {
	repo := &fakeRepo{}
	uc := settings.New(repo, nil)

	_, err := uc.Update(context.Background(), "Mars/Phobos", settings.ProviderDeepSeek)
	require.Error(t, err)
	assert.True(t, derr.IsValidation(err))
	// 設定は変更されない
	assert.Nil(t, repo.current)
}

func TestUpdate_EmptyTimezone(t *testing.T) {
	uc := settings.New(&fakeRepo{}, nil)
	_, err := uc.Update(context.Background(), "", settings.ProviderDeepSeek)
	assert.True(t, derr.IsValidation(err))
}
