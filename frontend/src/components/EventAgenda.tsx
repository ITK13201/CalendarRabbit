import { useMemo } from 'react'
import { addMonths, endOfMonth, format, isSameDay, startOfDay, startOfMonth } from 'date-fns'
import { ja } from 'date-fns/locale'
import type { NavigateAction } from 'react-big-calendar'
import type { CalendarEvent } from '../api/types'

// カレンダー上で扱うイベント表現。CalendarScreen と共有する。
export interface RBCEvent {
  id: number
  title: string
  start: Date
  end: Date
  allDay: boolean
  resource: CalendarEvent
}

interface EventAgendaProps {
  date: Date
  events: RBCEvent[]
  onSelectEvent?: (event: RBCEvent) => void
}

interface DayGroup {
  day: Date
  items: RBCEvent[]
}

function timeLabel(event: RBCEvent): string {
  if (event.allDay) return '終日'
  if (isSameDay(event.start, event.end)) {
    return `${format(event.start, 'HH:mm')} 〜 ${format(event.end, 'HH:mm')}`
  }
  return `${format(event.start, 'M/d HH:mm')} 〜 ${format(event.end, 'M/d HH:mm')}`
}

// EventAgenda は react-big-calendar 標準の Agenda ビューを置き換えるカスタムビュー。
// 標準ビューは複数日イベントを日ごとに複製表示するため、ここでは1イベント=1カードで表示する。
export function EventAgenda({ date, events, onSelectEvent }: EventAgendaProps) {
  const groups = useMemo<DayGroup[]>(() => {
    const from = startOfMonth(date)
    const to = endOfMonth(date)
    const inMonth = events
      .filter((e) => e.start <= to && e.end >= from)
      .slice()
      .sort((a, b) => a.start.getTime() - b.start.getTime())

    const map = new Map<string, DayGroup>()
    for (const e of inMonth) {
      // 月初より前に開始する予定は月初の日付グループにまとめる。
      const day = startOfDay(e.start < from ? from : e.start)
      const key = format(day, 'yyyy-MM-dd')
      const existing = map.get(key)
      if (existing) existing.items.push(e)
      else map.set(key, { day, items: [e] })
    }
    return [...map.values()]
  }, [date, events])

  if (groups.length === 0) {
    return (
      <div className="rbc-agenda-view flex min-h-0 flex-1 flex-col">
        <span className="rbc-agenda-empty p-4 text-muted">この期間に予定はありません</span>
      </div>
    )
  }

  return (
    <div className="rbc-agenda-view flex min-h-0 flex-1 flex-col overflow-auto">
      <div className="flex flex-col gap-4 p-1">
        {groups.map((group) => (
          <div key={format(group.day, 'yyyy-MM-dd')} className="flex flex-col gap-2">
            <div className="text-[0.8rem] font-semibold text-muted">
              {format(group.day, 'M月d日 (EEE)', { locale: ja })}
            </div>
            {group.items.map((event) => (
              <button
                key={event.id}
                type="button"
                onClick={() => onSelectEvent?.(event)}
                className="flex flex-col gap-1 rounded-lg border border-border bg-surface p-3 text-left shadow-sm transition active:opacity-80"
              >
                <span className="font-semibold">{event.title}</span>
                <span className="text-[0.8rem] text-muted">{timeLabel(event)}</span>
              </button>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

// react-big-calendar のカスタムビューが必要とする静的メソッド群。月単位で扱う。
EventAgenda.title = (date: Date) => format(date, 'yyyy年M月', { locale: ja })

EventAgenda.range = (date: Date) => ({ start: startOfMonth(date), end: endOfMonth(date) })

EventAgenda.navigate = (date: Date, action: NavigateAction) => {
  switch (action) {
    case 'PREV':
      return addMonths(date, -1)
    case 'NEXT':
      return addMonths(date, 1)
    default:
      return date
  }
}
