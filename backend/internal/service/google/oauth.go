package google

import (
	"context"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// factory は oauth2.Config を用いた ClientFactory 実装。
type factory struct {
	cfg *oauth2.Config
}

// NewFactory は OAuth 設定から ClientFactory を生成する。
// スコープはイベント読み書きに必要な最小限（calendar）とする（design.md D2）。
func NewFactory(clientID, clientSecret, redirectURL string) ClientFactory {
	return &factory{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{calendar.CalendarScope},
			Endpoint:     googleoauth.Endpoint,
		},
	}
}

// AuthCodeURL は offline access（refresh token 取得）と consent 強制付きの認可 URL を返す。
func (f *factory) AuthCodeURL(state string) string {
	return f.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	)
}

// Exchange は認可コードをトークンへ交換する。
func (f *factory) Exchange(ctx context.Context, code string) (*Token, error) {
	tok, err := f.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, classifyErr(err)
	}
	return &Token{
		RefreshToken: tok.RefreshToken,
		AccessToken:  tok.AccessToken,
		Expiry:       tok.Expiry,
	}, nil
}

// Client は refresh token から自動更新される token source を作り、認証済みクライアントを返す。
func (f *factory) Client(ctx context.Context, refreshToken string) (CalendarClient, error) {
	tokenSource := f.cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	return newService(ctx, option.WithTokenSource(tokenSource))
}
