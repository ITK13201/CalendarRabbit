import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { LLMProvider } from '../api/types'

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

      <footer className="mt-auto pt-4 text-xs text-[var(--color-text-muted,#9ca3af)]">
        CalendarRabbit v{__APP_VERSION__}
      </footer>
    </section>
  )
}
