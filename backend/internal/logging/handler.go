package logging

import (
	"context"
	"log/slog"
)

// ログのトップレベルに昇格する予約キー（design D1/D2）。
const (
	// MethodField は HTTP メソッドのトップレベルキー。
	MethodField = "method"
	// PathField は HTTP パスのトップレベルキー。
	PathField = "path"
	// QueryField は HTTP クエリのトップレベルキー。
	QueryField = "query"
	// ExtraGroup は予約キー以外の任意属性をネストするグループ名。
	ExtraGroup = "extra"
)

// contextHandler は slog.Handler をラップし、context 由来の
// requestId/method/path/query をトップレベルへ昇格しつつ、
// それ以外のすべての属性を extra グループへネストする（design D1/D2）。
type contextHandler struct {
	inner        slog.Handler
	groups       []string
	preformatted []slog.Attr
}

// NewContextHandler は inner をラップした contextHandler を返す。
func NewContextHandler(inner slog.Handler) slog.Handler {
	return &contextHandler{inner: inner}
}

// Enabled は inner に委譲する。
func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle は context 由来の予約キーをトップレベルへ、その他の属性を extra へ再構成する。
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	top := topLevelAttrs(ctx)

	extra := make([]slog.Attr, 0, len(h.preformatted)+r.NumAttrs())
	extra = append(extra, h.preformatted...)
	if r.NumAttrs() > 0 {
		recordAttrs := make([]slog.Attr, 0, r.NumAttrs())
		r.Attrs(func(a slog.Attr) bool {
			recordAttrs = append(recordAttrs, a)
			return true
		})
		extra = append(extra, wrapGroups(h.groups, recordAttrs)...)
	}

	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	nr.AddAttrs(top...)
	if len(extra) > 0 {
		nr.AddAttrs(slog.Attr{Key: ExtraGroup, Value: slog.GroupValue(extra...)})
	}
	return h.inner.Handle(ctx, nr)
}

// WithAttrs は付与された属性を（現在のグループ階層でネストして）extra 側へ蓄積する。
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	nh := h.clone()
	nh.preformatted = append(nh.preformatted, wrapGroups(h.groups, attrs)...)
	return nh
}

// WithGroup は以降の属性をネストするグループ名を積む。
func (h *contextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	nh.groups = append(nh.groups, name)
	return nh
}

func (h *contextHandler) clone() *contextHandler {
	return &contextHandler{
		inner:        h.inner,
		groups:       h.groups[:len(h.groups):len(h.groups)],
		preformatted: h.preformatted[:len(h.preformatted):len(h.preformatted)],
	}
}

// wrapGroups は attrs を groups の階層で内側からネストする。groups が空ならそのまま返す。
func wrapGroups(groups []string, attrs []slog.Attr) []slog.Attr {
	for i := len(groups) - 1; i >= 0; i-- {
		attrs = []slog.Attr{{Key: groups[i], Value: slog.GroupValue(attrs...)}}
	}
	return attrs
}

// topLevelAttrs は context からトップレベルへ昇格する予約キー属性を組み立てる。
// 値が空のキーは省略する（起動時など文脈を持たないログでは空になる）。
func topLevelAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attrs := make([]slog.Attr, 0, 4)
	if id := RequestIDFromContext(ctx); id != "" {
		attrs = append(attrs, slog.String(RequestIDField, id))
	}
	info := HTTPInfoFromContext(ctx)
	if info.Method != "" {
		attrs = append(attrs, slog.String(MethodField, info.Method))
	}
	if info.Path != "" {
		attrs = append(attrs, slog.String(PathField, info.Path))
	}
	if info.Query != "" {
		attrs = append(attrs, slog.String(QueryField, info.Query))
	}
	return attrs
}
