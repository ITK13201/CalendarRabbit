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

    // 楽観的表示: 送信内容を暫定メッセージとして即座に会話へ追加する。
    // サーバの正の id と衝突しないよう一時 id は負値を用いる。
    const optimistic: ChatMessage = {
      id: -Date.now(),
      role: 'user',
      content,
      created_at: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, optimistic])
    setLoading(true)
    try {
      await api.sendMessage(content)
      // 成功時はサーバ履歴で全置換し、暫定メッセージとの整合を取る。
      await reload()
    } catch (e) {
      // 失敗時は暫定メッセージを取り除き、入力内容を復元する。
      setMessages((prev) => prev.filter((m) => m.id !== optimistic.id))
      setInput(content)
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }

  async function handleClear() {
    // 誤操作防止のため確認を挟み、承諾時のみ削除する。
    if (!window.confirm('会話履歴と提示中の予定案をすべて削除します。よろしいですか？')) return
    setError(null)
    setNotice(null)
    try {
      await api.clearConversation()
      setMessages([])
      setProposals([])
      setNotice('会話をクリアしました')
    } catch (e) {
      setError(String(e))
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
    <section className="flex min-h-0 min-w-0 flex-1 flex-col gap-[14px] p-4 pb-0">
      <header className="flex items-center justify-between">
        <h1 className="m-0 text-[1.35rem] font-bold tracking-[-0.01em]">チャット</h1>
        <button
          type="button"
          className="btn btn-secondary"
          onClick={handleClear}
          disabled={loading || messages.length === 0}
        >
          会話をクリア
        </button>
      </header>

      <div className="flex flex-1 flex-col gap-[10px] overflow-y-auto p-1" ref={listRef}>
        {messages.length === 0 && !loading && (
          <p className="mt-10 text-center text-[0.9rem] text-muted">
            「TGSの予定を追加して」のように話しかけてください。
          </p>
        )}
        {messages.map((m) => (
          <div key={m.id} className={`bubble bubble-${m.role}`}>
            <div>{m.content}</div>
            <div className="bubble-time">{formatDateTime(m.created_at)}</div>
          </div>
        ))}
        {loading && <div className="bubble bubble-assistant loading">検索中…</div>}
      </div>

      {proposals.filter((p) => p.status === 'pending').length > 0 && (
        <div className="flex flex-col gap-[10px]">
          {proposals
            .filter((p) => p.status === 'pending')
            .map((p) => (
              <div
                key={p.id}
                className="rounded-md border border-border border-l-[3px] border-l-primary bg-surface p-[14px] shadow-sm"
              >
                <div className="text-[0.95rem] font-bold">{p.title}</div>
                <div className="mt-0.5 text-[0.85rem] text-muted">
                  {formatDateTime(p.starts_at)} 〜 {formatDateTime(p.ends_at)}
                </div>
                {p.location && <div className="mt-0.5 text-[0.85rem] text-muted">📍 {p.location}</div>}
                {p.source_url && (
                  <a
                    className="text-[0.85rem] font-medium text-primary no-underline hover:underline"
                    href={p.source_url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    情報源
                  </a>
                )}
                <div className="mt-3 flex gap-2">
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

      <div className="flex gap-2 px-0 pt-[10px] pb-[calc(10px+env(safe-area-inset-bottom))]">
        <input
          type="text"
          value={input}
          placeholder="メッセージを入力"
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSend()
          }}
          disabled={loading}
          className="flex-1 rounded-full border border-border-strong bg-surface px-[14px] py-[11px] transition focus:border-primary focus:outline-none focus:ring-3 focus:ring-primary-ring"
        />
        <button type="button" className="btn btn-primary" onClick={handleSend} disabled={loading}>
          送信
        </button>
      </div>
    </section>
  )
}
