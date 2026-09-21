import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { GoogleStatus, LLMProvider } from '../api/types'

// 代表的なタイムゾーン候補（必要に応じて拡張可能）。
const TIMEZONES = [
  'Asia/Tokyo',
  'UTC',
  'America/Los_Angeles',
  'America/New_York',
  'Europe/London',
  'Europe/Paris',
  'Asia/Shanghai',
  'Asia/Seoul',
  'Australia/Sydney',
]

// LLM プロバイダの選択肢（既定は deepseek）。
const LLM_PROVIDERS: { value: LLMProvider; label: string }[] = [
  { value: 'deepseek', label: 'DeepSeek（既定・低コスト）' },
  { value: 'claude', label: 'Claude（高精度・ネイティブweb検索）' },
]

export function SettingsScreen() {
  const [timezone, setTimezone] = useState('Asia/Tokyo')
  const [llmProvider, setLlmProvider] = useState<LLMProvider>('deepseek')
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api
      .getSettings()
      .then((s) => {
        setTimezone(s.timezone)
        setLlmProvider(s.llm_provider)
        setLoaded(true)
      })
      .catch((e) => setError(String(e)))
  }, [])

  async function save() {
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const updated = await api.updateSettings(timezone, llmProvider)
      setTimezone(updated.timezone)
      setLlmProvider(updated.llm_provider)
      setNotice('設定を保存しました')
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const options = TIMEZONES.includes(timezone) ? TIMEZONES : [timezone, ...TIMEZONES]

  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col gap-[14px] p-4">
      <header className="flex items-center justify-between">
        <h1 className="m-0 text-[1.35rem] font-bold tracking-[-0.01em]">設定</h1>
      </header>

      <label className="field">
        <span>タイムゾーン</span>
        <select value={timezone} onChange={(e) => setTimezone(e.target.value)} disabled={!loaded}>
          {options.map((tz) => (
            <option key={tz} value={tz}>
              {tz}
            </option>
          ))}
        </select>
      </label>

      <label className="field">
        <span>予定抽出に使うLLM</span>
        <select
          value={llmProvider}
          onChange={(e) => setLlmProvider(e.target.value as LLMProvider)}
          disabled={!loaded}
        >
          {LLM_PROVIDERS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </select>
      </label>

      <button
        type="button"
        className="btn btn-primary self-start"
        onClick={save}
        disabled={busy || !loaded}
      >
        保存
      </button>

      {notice && <div className="notice">{notice}</div>}
      {error && <div className="error">{error}</div>}

      <GoogleSyncSection />

      <footer className="mt-auto pt-4 text-xs text-[var(--color-text-muted,#9ca3af)]">
        CalendarRabbit v{__APP_VERSION__}
      </footer>
    </section>
  )
}

// GoogleSyncSection は Google Calendar 連携の状態表示と接続/解除/再同期 UI を提供する。
// GET /api/google/status の configured=false のとき（連携が未設定のサーバ）は何も表示しない。
function GoogleSyncSection() {
  const [status, setStatus] = useState<GoogleStatus | null>(null)
  const [deleteCalendar, setDeleteCalendar] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  async function refresh() {
    try {
      setStatus(await api.getGoogleStatus())
    } catch (e) {
      setError(String(e))
    }
  }

  useEffect(() => {
    refresh()
    // コールバックからの復帰通知（App が付与する ?google=connected|error）。
    const params = new URLSearchParams(window.location.search)
    const g = params.get('google')
    if (g === 'connected') setNotice('Google Calendar と連携しました')
    else if (g === 'error') setError('Google Calendar の連携に失敗しました')
  }, [])

  async function connect() {
    setBusy(true)
    setError(null)
    try {
      const { auth_url } = await api.getGoogleAuthURL()
      window.location.href = auth_url
    } catch (e) {
      setError(String(e))
      setBusy(false)
    }
  }

  async function disconnect() {
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      await api.disconnectGoogle(deleteCalendar)
      setNotice('連携を解除しました')
      await refresh()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  async function resync() {
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const { pending_count } = await api.resyncGoogle()
      setNotice(`再同期しました（未同期: ${pending_count}件）`)
      await refresh()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  // 連携が未設定のサーバでは何も表示しない。
  if (!status || !status.configured) return null

  return (
    <section className="flex flex-col gap-[10px] border-t border-[var(--color-border,#e5e7eb)] pt-4">
      <h2 className="m-0 text-[1.05rem] font-semibold">Google Calendar 連携</h2>

      <p className="m-0 text-sm">
        状態:{' '}
        {status.connected ? (
          <span className="font-semibold text-[var(--color-accent,#16a34a)]">連携済み</span>
        ) : (
          <span className="text-[var(--color-text-muted,#9ca3af)]">未連携</span>
        )}
      </p>

      {status.needs_reconnect && (
        <div className="error">
          トークンの失効、または専用カレンダーの削除を検知しました。再連携してください。
        </div>
      )}

      {status.connected && (
        <p className="m-0 text-sm text-[var(--color-text-muted,#9ca3af)]">
          未同期: {status.pending_count}件
        </p>
      )}

      {!status.connected ? (
        <button type="button" className="btn btn-primary self-start" onClick={connect} disabled={busy}>
          {status.needs_reconnect ? '再連携' : '接続'}
        </button>
      ) : (
        <div className="flex flex-col gap-[10px]">
          <button type="button" className="btn self-start" onClick={resync} disabled={busy}>
            再同期
          </button>

          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={deleteCalendar}
              onChange={(e) => setDeleteCalendar(e.target.checked)}
            />
            <span>解除時に Google 上の専用カレンダーごと削除する（既定は残す）</span>
          </label>
          <button
            type="button"
            className="btn btn-danger self-start"
            onClick={disconnect}
            disabled={busy}
          >
            解除
          </button>
        </div>
      )}

      {notice && <div className="notice">{notice}</div>}
      {error && <div className="error">{error}</div>}
    </section>
  )
}
