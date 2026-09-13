import type {
  AppSettings,
  CalendarEvent,
  ConversationResponse,
  EventInput,
  EventProposal,
  SendMessageResponse,
} from './types'

const BASE = '/api'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    let message = `request failed: ${res.status}`
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      // ignore parse error
    }
    throw new Error(message)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

export const api = {
  // calendar
  listEvents(from?: Date, to?: Date): Promise<CalendarEvent[]> {
    const params = new URLSearchParams()
    if (from) params.set('from', from.toISOString())
    if (to) params.set('to', to.toISOString())
    const qs = params.toString()
    return request<CalendarEvent[]>(`/calendar/events${qs ? `?${qs}` : ''}`)
  },
  createEvent(input: EventInput): Promise<CalendarEvent> {
    return request<CalendarEvent>('/calendar/events', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updateEvent(id: number, input: EventInput): Promise<CalendarEvent> {
    return request<CalendarEvent>(`/calendar/events/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    })
  },
  deleteEvent(id: number): Promise<void> {
    return request<void>(`/calendar/events/${id}`, { method: 'DELETE' })
  },

  // chat
  sendMessage(content: string): Promise<SendMessageResponse> {
    return request<SendMessageResponse>('/chat/messages', {
      method: 'POST',
      body: JSON.stringify({ content }),
    })
  },
  getConversation(): Promise<ConversationResponse> {
    return request<ConversationResponse>('/chat/conversations')
  },
  approveProposal(id: number, edited?: EventInput): Promise<CalendarEvent> {
    return request<CalendarEvent>(`/chat/proposals/${id}/approve`, {
      method: 'POST',
      body: JSON.stringify(edited ? { edited } : {}),
    })
  },
  rejectProposal(id: number): Promise<EventProposal> {
    return request<EventProposal>(`/chat/proposals/${id}/reject`, {
      method: 'POST',
    })
  },

  // settings
  getSettings(): Promise<AppSettings> {
    return request<AppSettings>('/settings')
  },
  updateSettings(timezone: string): Promise<AppSettings> {
    return request<AppSettings>('/settings', {
      method: 'PUT',
      body: JSON.stringify({ timezone }),
    })
  },
}
