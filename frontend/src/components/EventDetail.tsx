import { useState } from 'react'
import type { CalendarEvent } from '../api/types'

interface EventDetailProps {
  event: CalendarEvent
  onEdit: () => void
  onDelete: () => Promise<void>
  onClose: () => void
}

function formatRange(event: CalendarEvent): string {
  const start = new Date(event.starts_at)
  const end = new Date(event.ends_at)
  if (event.all_day) {
    return `${start.toLocaleDateString('ja-JP', { dateStyle: 'medium' })}（終日）`
  }
  const opts: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' }
  return `${start.toLocaleString('ja-JP', opts)} 〜 ${end.toLocaleString('ja-JP', opts)}`
}

// EventDetail はカレンダー上の予定を読み取り専用で表示するモーダル。
// ここからの明示操作でのみ編集・削除へ遷移する。
export function EventDetail({ event, onEdit, onDelete, onClose }: EventDetailProps) {
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function handleDelete() {
    if (!window.confirm('この予定を削除します。よろしいですか？')) return
    setBusy(true)
    setError(null)
    try {
      await onDelete()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true">
      <div className="modal">
        <h2>{event.title}</h2>

        <div className="flex flex-col gap-[3px] text-[0.9rem]">
          <span className="text-[0.72rem] font-semibold uppercase tracking-[0.03em] text-muted">日時</span>
          <span className="break-words">{formatRange(event)}</span>
        </div>
        {event.location && (
          <div className="flex flex-col gap-[3px] text-[0.9rem]">
            <span className="text-[0.72rem] font-semibold uppercase tracking-[0.03em] text-muted">場所</span>
            <span className="break-words">📍 {event.location}</span>
          </div>
        )}
        {event.description && (
          <div className="flex flex-col gap-[3px] text-[0.9rem]">
            <span className="text-[0.72rem] font-semibold uppercase tracking-[0.03em] text-muted">概要</span>
            <span className="whitespace-pre-wrap break-words">{event.description}</span>
          </div>
        )}
        {event.source_url && (
          <div className="flex flex-col gap-[3px] text-[0.9rem]">
            <span className="text-[0.72rem] font-semibold uppercase tracking-[0.03em] text-muted">情報源</span>
            <a
              className="break-words text-primary"
              href={event.source_url}
              target="_blank"
              rel="noreferrer"
            >
              {event.source_url}
            </a>
          </div>
        )}

        {error && <div className="error">{error}</div>}

        <div className="modal-actions">
          <button type="button" className="btn btn-primary" onClick={onEdit} disabled={busy}>
            編集
          </button>
          <button type="button" className="btn btn-danger" onClick={handleDelete} disabled={busy}>
            削除
          </button>
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={busy}>
            閉じる
          </button>
        </div>
      </div>
    </div>
  )
}
