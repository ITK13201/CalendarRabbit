import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { BottomTabs } from './BottomTabs'

describe('BottomTabs', () => {
  it('3つのタブを表示しアクティブタブを識別できる', () => {
    render(<BottomTabs active="chat" onChange={() => {}} />)
    expect(screen.getByText('チャット')).toBeInTheDocument()
    expect(screen.getByText('カレンダー')).toBeInTheDocument()
    expect(screen.getByText('設定')).toBeInTheDocument()

    const chatTab = screen.getByText('チャット').closest('button')
    expect(chatTab).toHaveAttribute('aria-current', 'page')
  })

  it('タブ選択で onChange が呼ばれる', () => {
    const onChange = vi.fn()
    render(<BottomTabs active="chat" onChange={onChange} />)
    fireEvent.click(screen.getByText('カレンダー'))
    expect(onChange).toHaveBeenCalledWith('calendar')
  })
})
