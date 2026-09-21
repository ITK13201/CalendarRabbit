package persistence

import (
	"context"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/googleconnection"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// TokenCipher は refresh token の暗号化/復号を抽象化する（実装は service/google.Cipher）。
// persistence 層が暗号実装へ直接依存しないためのインターフェース。
type TokenCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// GoogleConnectionInput は連携状態の保存に用いる入力。RefreshToken は平文で渡す。
type GoogleConnectionInput struct {
	RefreshToken string
	AccessToken  string
	TokenExpiry  time.Time
	CalendarID   string
	Connected    bool
}

// GoogleConnectionRepository は GoogleConnection（単一レコード）の永続化を担う。
// refresh token は保存時に暗号化し、取得時に復号する（design.md D3b）。
type GoogleConnectionRepository struct {
	client *ent.Client
	cipher TokenCipher
	logger *slog.Logger
}

// NewGoogleConnectionRepository は GoogleConnectionRepository を生成する。
func NewGoogleConnectionRepository(client *ent.Client, cipher TokenCipher, logger *slog.Logger) *GoogleConnectionRepository {
	return &GoogleConnectionRepository{client: client, cipher: cipher, logger: logger}
}

// Get は連携状態を取得し refresh token を復号して返す。未保存なら derr.ErrNotFound。
func (r *GoogleConnectionRepository) Get(ctx context.Context) (res *entity.GoogleConnection, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.GoogleConnectionRepository.Get", nil, &res, &err)()

	row, err := r.client.GoogleConnection.Query().
		Order(ent.Asc(googleconnection.FieldID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}

	refreshToken := ""
	if row.RefreshToken != "" {
		refreshToken, err = r.cipher.Decrypt(row.RefreshToken)
		if err != nil {
			return nil, err
		}
	}
	return mapGoogleConnection(row, refreshToken), nil
}

// Save は連携状態を作成または更新する（単一レコード運用）。refresh token は暗号化して保存する。
func (r *GoogleConnectionRepository) Save(ctx context.Context, in GoogleConnectionInput) (res *entity.GoogleConnection, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.GoogleConnectionRepository.Save", logging.Args{"calendarID": in.CalendarID, "connected": in.Connected}, &res, &err)()

	encRefresh := ""
	if in.RefreshToken != "" {
		encRefresh, err = r.cipher.Encrypt(in.RefreshToken)
		if err != nil {
			return nil, err
		}
	}

	existing, err := r.client.GoogleConnection.Query().
		Order(ent.Asc(googleconnection.FieldID)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if ent.IsNotFound(err) {
		create := r.client.GoogleConnection.Create().
			SetRefreshToken(encRefresh).
			SetAccessToken(in.AccessToken).
			SetCalendarID(in.CalendarID).
			SetConnected(in.Connected)
		if !in.TokenExpiry.IsZero() {
			create = create.SetTokenExpiry(in.TokenExpiry.UTC())
		}
		created, cerr := create.Save(ctx)
		if cerr != nil {
			return nil, cerr
		}
		return mapGoogleConnection(created, in.RefreshToken), nil
	}

	update := r.client.GoogleConnection.UpdateOneID(existing.ID).
		SetRefreshToken(encRefresh).
		SetAccessToken(in.AccessToken).
		SetCalendarID(in.CalendarID).
		SetConnected(in.Connected)
	if in.TokenExpiry.IsZero() {
		update = update.ClearTokenExpiry()
	} else {
		update = update.SetTokenExpiry(in.TokenExpiry.UTC())
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return mapGoogleConnection(updated, in.RefreshToken), nil
}

// MarkDisconnected は連携を未連携相当に落とす（トークン失効・カレンダー消失時）。
// connected=false とし、トークン類を破棄する。clearCalendar=true の場合は calendar_id も破棄する
// （専用カレンダーが手動削除された場合は再連携で作り直すため）。レコード自体は残し、
// Status が「再連携が必要」を判別できるようにする。未保存時は何もしない。
func (r *GoogleConnectionRepository) MarkDisconnected(ctx context.Context, clearCalendar bool) (err error) {
	defer logging.Trace(ctx, r.logger, "persistence.GoogleConnectionRepository.MarkDisconnected", logging.Args{"clearCalendar": clearCalendar}, nil, &err)()

	existing, err := r.client.GoogleConnection.Query().
		Order(ent.Asc(googleconnection.FieldID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return err
	}

	update := r.client.GoogleConnection.UpdateOneID(existing.ID).
		SetConnected(false).
		SetRefreshToken("").
		SetAccessToken("").
		ClearTokenExpiry()
	if clearCalendar {
		update = update.SetCalendarID("")
	}
	return update.Exec(ctx)
}

// Clear は連携状態を破棄する（保存済みトークン・calendarId を含む全レコードを削除）。
func (r *GoogleConnectionRepository) Clear(ctx context.Context) (err error) {
	defer logging.Trace(ctx, r.logger, "persistence.GoogleConnectionRepository.Clear", nil, nil, &err)()

	_, err = r.client.GoogleConnection.Delete().Exec(ctx)
	return err
}

func mapGoogleConnection(row *ent.GoogleConnection, refreshToken string) *entity.GoogleConnection {
	conn := &entity.GoogleConnection{
		ID:           row.ID,
		RefreshToken: refreshToken,
		AccessToken:  row.AccessToken,
		CalendarID:   row.CalendarID,
		Connected:    row.Connected,
		CreatedAt:    row.CreatedAt.UTC(),
		UpdatedAt:    row.UpdatedAt.UTC(),
	}
	if !row.TokenExpiry.IsZero() {
		conn.TokenExpiry = row.TokenExpiry.UTC()
	}
	return conn
}
