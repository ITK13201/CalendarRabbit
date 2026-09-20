package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"time"
)

// Args は計装ヘルパ Trace へ渡す引数マップ。
// CLAUDE.md の「any を避ける」方針の例外として、計装基盤の内部境界に限り any を許容する。
type Args map[string]any

// Trace は defer ベースの計装ヘルパ（design D3）。
// 呼び出し時に "[name] started" を即時出力し、返り関数を defer 実行すると
// "[name] finished" を出力する。res/errp はポインタで受け取り、return 確定後の値を読む。
//
//	func (u *UseCase) Do(ctx context.Context, in In) (res *Out, err error) {
//	    defer logging.Trace(ctx, u.logger, "pkg.UseCase.Do", logging.Args{"in": in}, &res, &err)()
//	    ...
//	}
//
// res には名前付き戻り値のポインタ（&res）を、errp には &err を渡す。戻り値が
// 不要な場合（error のみを返すメソッド等）は res に nil を渡してよい。
// args・result には D6 マスクと Truncate（1000文字）を一律適用する。
func Trace(ctx context.Context, logger *slog.Logger, name string, args Args, res any, errp *error) func() {
	if logger == nil {
		logger = Default()
	}
	start := time.Now()
	safeArgs := sanitize(map[string]any(args))

	logger.LogAttrs(ctx, slog.LevelInfo, name+" started", slog.Any("args", safeArgs))

	return func() {
		attrs := []slog.Attr{
			slog.Any("args", safeArgs),
			slog.Float64("latency_ms", DurationMillis(time.Since(start))),
		}
		if res != nil {
			attrs = append(attrs, slog.Any("result", sanitize(deref(res))))
		}
		level := slog.LevelInfo
		if errp != nil && *errp != nil {
			level = slog.LevelError
			attrs = append(attrs, slog.String("error", (*errp).Error()))
		}
		logger.LogAttrs(ctx, level, name+" finished", attrs...)
	}
}

// DurationMillis は Duration をミリ秒（サブミリ秒精度の float）へ変換する。
// Milliseconds() の整数切り捨てで 1ms 未満が 0 になる問題を避ける。
func DurationMillis(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// deref はポインタを1段だけ辿った値を返す。名前付き戻り値のポインタ（&res）から
// return 確定後の実値を取り出すために用いる。
func deref(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		return rv.Elem().Interface()
	}
	return v
}

// sanitize は任意の値を JSON 経由で map/slice/文字列へ正規化した上で、
// 機密キーのマスク（D6）と文字列の Truncate（1000文字）を再帰適用する。
// JSON 化できない値は文字列表現へフォールバックして丸める。
func sanitize(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return Truncate(fmt.Sprintf("%v", v), MaxTruncateRunes)
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return Truncate(string(b), MaxTruncateRunes)
	}
	return sanitizeValue(decoded)
}

// sanitizeValue は正規化済みの値へマスクと丸めを再帰適用する。
func sanitizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if IsSensitiveKey(k) {
				out[k] = RedactedValue
				continue
			}
			out[k] = sanitizeValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = sanitizeValue(val)
		}
		return out
	case string:
		return Truncate(t, MaxTruncateRunes)
	default:
		return v
	}
}
