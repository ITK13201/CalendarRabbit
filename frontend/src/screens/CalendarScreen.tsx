import { useCallback, useEffect, useMemo, useState } from 'react'
import { Calendar, dateFnsLocalizer, type View } from 'react-big-calendar'
import { format, parse, startOfWeek, getDay, startOfMonth, endOfMonth } from 'date-fns'
import { ja } from 'date-fns/locale'
import 'react-big-calendar/lib/css/react-big-calendar.css'
import { api } from '../api/client'
import type { CalendarEvent, EventInput } from '../api/types'
import { EventForm } from '../components/EventForm'

const locales = { ja }
const localizer = dateFnsLocalizer({
  format,
  parse,
  startOfWeek: () => startOfWeek(new Date(), { weekStartsOn: 0 }),
  getDay,
  locales,
})

interface RBCEvent {
  id: number
  title: string
  start: Date
  end: Date
  allDay: boolean
  resource: CalendarEvent
}

export function CalendarScreen() {
  const [current, setCurrent] = useState(new Date())
  const [view, setView] = useState<View>('month')
  const [events, setEvents] = useState<CalendarEvent[]>([])
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<CalendarEvent | null>(null)
  const [creating, setCreating] = useState<{ start: Date; end: Date } | null>(null)

  const load = useCallback(async (date: Date) => {
    try {
      const from = startOfMonth(date)
      const to = endOfMonth(date)
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
        start: new Date(e.starts_at),
        end: new Date(e.ends_at),
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

  async function handleDelete() {
    if (!editing) return
    await api.deleteEvent(editing.id)
    setEditing(null)
    await load(current)
  }

  return (
    <section className="screen calendar-screen">
      <header className="screen-header">
        <h1>カレンダー</h1>
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

      <div className="calendar-container">
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
          views={['month', 'agenda']}
          style={{ height: '100%' }}
          onSelectEvent={(e: RBCEvent) => setEditing(e.resource)}
          selectable
          onSelectSlot={(slot) => setCreating({ start: slot.start, end: slot.end })}
          messages={{
            next: '次',
            previous: '前',
            today: '今日',
            month: '月',
            agenda: '一覧',
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
          onDelete={handleDelete}
        />
      )}
    </section>
  )
}
