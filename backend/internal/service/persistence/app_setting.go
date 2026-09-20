package persistence

import (
	"context"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/appsetting"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
)

// AppSettingInput は設定の作成／更新に用いる入力。
type AppSettingInput struct {
	Timezone    string
	LLMProvider string
}

// AppSettingRepository は AppSetting（単一レコード）の永続化を担う。
// 単一ユーザー・単一カレンダー前提のため、常に最古の1レコードを設定として扱う。
type AppSettingRepository struct {
	client *ent.Client
}

func NewAppSettingRepository(client *ent.Client) *AppSettingRepository {
	return &AppSettingRepository{client: client}
}

// Get は設定レコードを取得する。未初期化なら derr.ErrNotFound を返す。
func (r *AppSettingRepository) Get(ctx context.Context) (*entity.AppSetting, error) {
	row, err := r.client.AppSetting.Query().
		Order(ent.Asc(appsetting.FieldID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return mapAppSetting(row), nil
}

// Upsert は設定レコードを作成または更新する。
func (r *AppSettingRepository) Upsert(ctx context.Context, in AppSettingInput) (*entity.AppSetting, error) {
	provider := appsetting.LlmProvider(in.LLMProvider)
	existing, err := r.client.AppSetting.Query().
		Order(ent.Asc(appsetting.FieldID)).
		First(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return nil, err
		}
		created, cerr := r.client.AppSetting.Create().
			SetTimezone(in.Timezone).
			SetLlmProvider(provider).
			Save(ctx)
		if cerr != nil {
			return nil, cerr
		}
		return mapAppSetting(created), nil
	}
	updated, err := r.client.AppSetting.UpdateOneID(existing.ID).
		SetTimezone(in.Timezone).
		SetLlmProvider(provider).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return mapAppSetting(updated), nil
}

func mapAppSetting(row *ent.AppSetting) *entity.AppSetting {
	return &entity.AppSetting{
		ID:          row.ID,
		Timezone:    row.Timezone,
		LLMProvider: string(row.LlmProvider),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}
