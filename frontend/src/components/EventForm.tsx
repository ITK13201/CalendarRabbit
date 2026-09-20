import { useState } from 'react'
import DatePicker, { registerLocale } from 'react-datepicker'
import { ja } from 'date-fns/locale'
import 'react-datepicker/dist/react-datepicker.css'
import type { EventInput } from '../api/types'

registerLocale('ja', ja)

// 入力/表示に用いる日時フォーマット（24時間表記）。
const DATETIME_FORMAT = 'yyyy/MM/dd, HH:mm'

interface EventFormProps {
  title: string
  initial: EventInput
  onSubmit: (input: EventInput) => Promise<void>
  onCancel: () => void
  onDelete?: () => Promise<void>
}

// ISO文字列を Date へ変換（空なら null）。
function toDate(iso: string): Date | null {
  if (!iso) return null
  const d = new Date(iso)
  return isNaN(d.getTime()) ? null : d
}

export function EventForm({ title, initial, onSubmit, onCancel, onDelete }: EventFormProps) {
  const [form, setForm] = useState({
    title: initial.title,
    starts_at: toDate(initial.starts_at),
    ends_at: toDate(initial.ends_at),
    all_day: initial.all_day,
    location: initial.location,
    description: initial.description,
    source_url: initial.source_url,
  })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit() {
    if (!form.title.trim()) {
      setError('名称を入力してください')
      return
    }
    if (!form.starts_at || !form.ends_at) {
      setError('開始・終了日時を入力してください')
      return
    }
    setBusy(true)
    setError(null)
    try {
      await onSubmit({
        title: form.title,
        starts_at: form.starts_at.toISOString(),
        ends_at: form.ends_at.toISOString(),
        all_day: form.all_day,
        location: form.location,
        description: form.description,
        source_url: form.source_url,
      })
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true">
      <div className="modal">
        <h2>{title}</h2>
        <label className="field">
          <span>名称</span>
          <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
        </label>
        <label className="field">
          <span>開始</span>
          <DatePicker
            selected={form.starts_at}
            onChange={(d: Date | null) => setForm({ ...form, starts_at: d })}
            showTimeSelect
            timeFormat="HH:mm"
            timeIntervals={15}
            timeCaption="時刻"
            dateFormat={DATETIME_FORMAT}
            locale="ja"
            placeholderText="yyyy/mm/dd, hh:mm"
            withPortal
          />
        </label>
        <label className="field">
          <span>終了</span>
          <DatePicker
            selected={form.ends_at}
            onChange={(d: Date | null) => setForm({ ...form, ends_at: d })}
            showTimeSelect
            timeFormat="HH:mm"
            timeIntervals={15}
            timeCaption="時刻"
            dateFormat={DATETIME_FORMAT}
            locale="ja"
            placeholderText="yyyy/mm/dd, hh:mm"
            minDate={form.starts_at ?? undefined}
            withPortal
          />
        </label>
        <label className="field field-inline">
          <input
            type="checkbox"
            checked={form.all_day}
            onChange={(e) => setForm({ ...form, all_day: e.target.checked })}
          />
          <span>終日</span>
        </label>
        <label className="field">
          <span>場所</span>
          <input value={form.location} onChange={(e) => setForm({ ...form, location: e.target.value })} />
        </label>
        <label className="field">
          <span>概要</span>
          <textarea value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
        </label>
        <label className="field">
          <span>情報源URL</span>
          <input value={form.source_url} onChange={(e) => setForm({ ...form, source_url: e.target.value })} />
        </label>

        {error && <div className="error">{error}</div>}

        <div className="modal-actions">
          <button type="button" className="btn btn-primary" onClick={submit} disabled={busy}>
            保存
          </button>
          {onDelete && (
            <button
              type="button"
              className="btn btn-danger"
              onClick={async () => {
                setBusy(true)
                try {
                  await onDelete()
                } catch (e) {
                  setError(String(e))
                } finally {
                  setBusy(false)
                }
              }}
              disabled={busy}
            >
              削除
            </button>
          )}
          <button type="button" className="btn btn-secondary" onClick={onCancel} disabled={busy}>
            キャンセル
          </button>
        </div>
      </div>
    </div>
  )
}
