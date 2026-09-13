import { useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { ChatMessage, EventProposal } from '../api/types'

function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('ja-JP', { dateStyle: 'medium', timeStyle: 'short' })
}

export function ChatScreen() {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [proposals, setProposals] = useState<EventProposal[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const listRef = useRef<HTMLDivElement>(null)

  async function reload() {
    const conv = await api.getConversation()
    setMessages(conv.messages)
    setProposals(conv.proposals)
  }

  useEffect(() => {
    reload().catch((e) => setError(String(e)))
  }, [])

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight })
  }, [messages, loading])

  async function handleSend() {
    const content = input.trim()
    if (!content || loading) return
    setInput('')
    setError(null)
    setNotice(null)
    setLoading(true)
    try {
      await api.sendMessage(content)
      await reload()
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }

  async function handleApprove(id: number) {
    setError(null)
    try {
      await api.approveProposal(id)
      setNotice('カレンダーに登録しました')
      await reload()
    } catch (e) {
      setError(String(e))
    }
  }

  async function handleReject(id: number) {
    setError(null)
    try {
      await api.rejectProposal(id)
      setNotice('予定案を却下しました')
      await reload()
    } catch (e) {
      setError(String(e))
    }
  }

  return (
    <section className="screen chat-screen">
      <header className="screen-header">
        <h1>チャット</h1>
      </header>

      <div className="chat-messages" ref={listRef}>
        {messages.length === 0 && !loading && (
          <p className="empty-hint">「TGSの予定を追加して」のように話しかけてください。</p>
        )}
        {messages.map((m) => (
          <div key={m.id} className={`bubble bubble-${m.role}`}>
            <div className="bubble-content">{m.content}</div>
            <div className="bubble-time">{formatDateTime(m.created_at)}</div>
          </div>
        ))}
        {loading && <div className="bubble bubble-assistant loading">検索中…</div>}
      </div>

      {proposals.filter((p) => p.status === 'pending').length > 0 && (
        <div className="proposals">
          {proposals
            .filter((p) => p.status === 'pending')
            .map((p) => (
              <div key={p.id} className="proposal-card">
                <div className="proposal-title">{p.title}</div>
                <div className="proposal-meta">
                  {formatDateTime(p.starts_at)} 〜 {formatDateTime(p.ends_at)}
                </div>
                {p.location && <div className="proposal-meta">📍 {p.location}</div>}
                {p.source_url && (
                  <a className="proposal-source" href={p.source_url} target="_blank" rel="noreferrer">
                    情報源
                  </a>
                )}
                <div className="proposal-actions">
                  <button type="button" className="btn btn-primary" onClick={() => handleApprove(p.id)}>
                    承認して登録
                  </button>
                  <button type="button" className="btn btn-secondary" onClick={() => handleReject(p.id)}>
                    却下
                  </button>
                </div>
              </div>
            ))}
        </div>
      )}

      {notice && <div className="notice">{notice}</div>}
      {error && <div className="error">{error}</div>}

      <div className="chat-input">
        <input
          type="text"
          value={input}
          placeholder="メッセージを入力"
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSend()
          }}
          disabled={loading}
        />
        <button type="button" className="btn btn-primary" onClick={handleSend} disabled={loading}>
          送信
        </button>
      </div>
    </section>
  )
}
