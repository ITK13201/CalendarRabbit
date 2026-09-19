import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { BottomTabs } from './BottomTabs'

describe('BottomTabs', () => {
  it('3つのタブをカレンダー→チャット→設定の順で表示しアクティブタブを識別できる', () => {
    render(<BottomTabs active="calendar" onChange={() => {}} />)

    const labels = screen.getAllByRole('button').map((b) => b.textContent)
    expect(labels).toEqual(['📅カレンダー', '💬チャット', '⚙️設定'])

    const calendarTab = screen.getByText('カレンダー').closest('button')
    expect(calendarTab).toHaveAttribute('aria-current', 'page')
  })

  it('タブ選択で onChange が呼ばれる', () => {
    const onChange = vi.fn()
    render(<BottomTabs active="chat" onChange={onChange} />)
    fireEvent.click(screen.getByText('カレンダー'))
    expect(onChange).toHaveBeenCalledWith('calendar')
  })
})
