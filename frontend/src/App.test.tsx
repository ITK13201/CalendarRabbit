import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { App } from './App'

// CalendarScreen はマウント時に api.listEvents を呼ぶためモックする。
vi.mock('./api/client', () => ({
  api: {
    listEvents: vi.fn().mockResolvedValue([]),
    getConversation: vi.fn().mockResolvedValue({ id: 1, messages: [], proposals: [] }),
  },
}))

describe('App', () => {
  it('起動時にカレンダー画面を表示する', async () => {
    render(<App />)
    // 既定画面はカレンダー（画面見出し h1「カレンダー」）。
    // マウント時の非同期ロード完了を待って act 警告を避ける。
    expect(await screen.findByRole('heading', { name: 'カレンダー' })).toBeInTheDocument()
    // チャット画面は初期表示されない
    expect(screen.queryByRole('heading', { name: 'チャット' })).not.toBeInTheDocument()
  })
})
