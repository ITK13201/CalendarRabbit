// バックエンド API のレスポンス型定義。

export interface CalendarEvent {
  id: number
  title: string
  starts_at: string
  ends_at: string
  all_day: boolean
  location: string
  description: string
  source_url: string
  created_at: string
  updated_at: string
}

export interface EventInput {
  title: string
  starts_at: string
  ends_at: string
  all_day: boolean
  location: string
  description: string
  source_url: string
}

export type MessageRole = 'user' | 'assistant'

export interface ChatMessage {
  id: number
  role: MessageRole
  content: string
  created_at: string
}

export type ProposalStatus = 'pending' | 'approved' | 'rejected'

export interface EventProposal {
  id: number
  title: string
  starts_at: string
  ends_at: string
  all_day: boolean
  location: string
  description: string
  source_url: string
  status: ProposalStatus
  calendar_event_id?: number
  created_at: string
  updated_at: string
}

export interface Candidate {
  title: string
  starts_at: string
  ends_at: string
  all_day: boolean
  location: string
  description: string
  source_url: string
}

export interface SendMessageResponse {
  user_message: ChatMessage
  assistant_message: ChatMessage
  proposal?: EventProposal
  candidates?: Candidate[]
}

export interface ConversationResponse {
  id: number
  messages: ChatMessage[]
  proposals: EventProposal[]
}

export type LLMProvider = 'deepseek' | 'claude'

export interface AppSettings {
  timezone: string
  llm_provider: LLMProvider
  updated_at?: string
}

export interface ApiError {
  error: string
  field?: string
}
