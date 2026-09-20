import { useCallback, useEffect, useMemo, useState } from 'react'
import { Calendar, dateFnsLocalizer, type View } from 'react-big-calendar'
import {
  format,
  parse,
  startOfWeek,
  endOfWeek,
  getDay,
  startOfDay,
  startOfMonth,
  endOfMonth,
  addDays,
} from 'date-fns'
import { ja } from 'date-fns/locale'
import 'react-big-calendar/lib/css/react-big-calendar.css'
import { api } from '../api/client'
import type { CalendarEvent, EventInput } from '../api/types'
import { EventForm } from '../components/EventForm'
import { EventDetail } from '../components/EventDetail'
import { EventAgenda, type RBCEvent } from '../components/EventAgenda'

const locales = { ja }
const localizer = dateFnsLocalizer({
  format,
  parse,
  startOfWeek: () => startOfWeek(new Date(), { weekStartsOn: 0 }),
  getDay,
  locales,
})

export function CalendarScreen() {
  const [current, setCurrent] = useState(new Date())
  const [view, setView] = useState<View>('month')
  const [events, setEvents] = useState<CalendarEvent[]>([])
  const [error, setError] = useState<string | null>(null)
  // viewing: 読み取り専用の詳細表示対象。editing: 編集フォーム対象。
  const [viewing, setViewing] = useState<CalendarEvent | null>(null)
  const [editing, setEditing] = useState<CalendarEvent | null>(null)
  const [creating, setCreating] = useState<{ start: Date; end: Date } | null>(null)

  const load = useCallback(async (date: Date) => {
    try {
      // 月ビューは前後の月の日も表示するため、表示グリッド全体（週境界まで）を取得範囲にする。
      // これにより月跨ぎ・グリッド端の予定も欠けずに取得できる。
      const from = startOfWeek(startOfMonth(date), { weekStartsOn: 0 })
      const to = endOfWeek(endOfMonth(date), { weekStartsOn: 0 })
      const list = await api.listEvents(from, to)
      setEvents(list)
      setError(null)
    } catch (e) {
      setError(String(e))
    }
  }, [])

  useEffect(() => {
    load(current)
  }, [current, load])

  const rbcEvents = useMemo<RBCEvent[]>(
    () =>
      events.map((e) => ({
        id: e.id,
        title: e.title,
        // 終日は日付のみで扱う（時刻成分でのズレを防ぐため 0 時に正規化）。
        start: e.all_day ? startOfDay(new Date(e.starts_at)) : new Date(e.starts_at),
        // react-big-calendar は終日イベントの end を排他的に扱う。ends_at は最終日を
        // 包括的に保持している（末尾時刻を含む場合もある）ため、最終日の 0 時へ正規化して
        // から +1 日し、最終日まで正しく描画させる。
        end: e.all_day ? addDays(startOfDay(new Date(e.ends_at)), 1) : new Date(e.ends_at),
        allDay: e.all_day,
        resource: e,
      })),
    [events],
  )

  async function handleCreate(input: EventInput) {
    await api.createEvent(input)
    setCreating(null)
    await load(current)
  }

  async function handleUpdate(input: EventInput) {
    if (!editing) return
    await api.updateEvent(editing.id, input)
    setEditing(null)
    await load(current)
  }

  async function handleDelete(id: number) {
    await api.deleteEvent(id)
    setEditing(null)
    setViewing(null)
    await load(current)
  }

  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col gap-[14px] p-4">
      <header className="flex items-center justify-between">
        <h1 className="m-0 text-[1.35rem] font-bold tracking-[-0.01em]">カレンダー</h1>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => {
            const start = new Date()
            const end = new Date(start.getTime() + 60 * 60 * 1000)
            setCreating({ start, end })
          }}
        >
          ＋新規
        </button>
      </header>

      {error && <div className="error">{error}</div>}

      <div className="calendar-container flex min-h-0 flex-1 flex-col rounded-lg border border-border bg-surface p-3 shadow-sm">
        <Calendar
          localizer={localizer}
          culture="ja"
          events={rbcEvents}
          startAccessor="start"
          endAccessor="end"
          date={current}
          view={view}
          onView={setView}
          onNavigate={(date) => setCurrent(date)}
          views={{ month: true, agenda: EventAgenda }}
          onSelectEvent={(e: RBCEvent) => setViewing(e.resource)}
          selectable
          onSelectSlot={(slot) => setCreating({ start: slot.start, end: slot.end })}
          messages={{
            next: '>',
            previous: '<',
            today: '今日',
            month: '月',
            agenda: '一覧',
            date: '日付',
            time: '時間',
            event: '予定',
            allDay: '終日',
            noEventsInRange: 'この期間に予定はありません',
          }}
        />
      </div>

      {creating && (
        <EventForm
          title="予定を作成"
          initial={{
            title: '',
            starts_at: creating.start.toISOString(),
            ends_at: creating.end.toISOString(),
            all_day: false,
            location: '',
            description: '',
            source_url: '',
          }}
          onSubmit={handleCreate}
          onCancel={() => setCreating(null)}
        />
      )}

      {viewing && (
        <EventDetail
          event={viewing}
          onEdit={() => {
            setEditing(viewing)
            setViewing(null)
          }}
          onDelete={() => handleDelete(viewing.id)}
          onClose={() => setViewing(null)}
        />
      )}

      {editing && (
        <EventForm
          title="予定を編集"
          initial={{
            title: editing.title,
            starts_at: editing.starts_at,
            ends_at: editing.ends_at,
            all_day: editing.all_day,
            location: editing.location,
            description: editing.description,
            source_url: editing.source_url,
          }}
          onSubmit={handleUpdate}
          onCancel={() => setEditing(null)}
          onDelete={() => handleDelete(editing.id)}
        />
      )}
    </section>
  )
}
