import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { SettingsScreen } from './SettingsScreen'
import { api } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    getSettings: vi.fn().mockResolvedValue({ timezone: 'Asia/Tokyo', llm_provider: 'deepseek' }),
    updateSettings: vi.fn(),
    getGoogleStatus: vi.fn(),
    getGoogleAuthURL: vi.fn(),
    disconnectGoogle: vi.fn(),
    resyncGoogle: vi.fn(),
  },
}))

const mockedApi = vi.mocked(api)

describe('SettingsScreen Google 連携', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('未設定（configured=false）のときは連携 UI を表示しない', async () => {
    mockedApi.getGoogleStatus.mockResolvedValue({ configured: false, connected: false, needs_reconnect: false, pending_count: 0 })
    render(<SettingsScreen />)
    await waitFor(() => expect(mockedApi.getGoogleStatus).toHaveBeenCalled())
    expect(screen.queryByText('Google Calendar 連携')).not.toBeInTheDocument()
  })

  it('未連携のときは「接続」ボタンを表示する', async () => {
    mockedApi.getGoogleStatus.mockResolvedValue({ configured: true, connected: false, needs_reconnect: false, pending_count: 0 })
    render(<SettingsScreen />)
    expect(await screen.findByRole('button', { name: '接続' })).toBeInTheDocument()
  })

  it('連携済みのときは状態・未同期件数・再同期/解除を表示する', async () => {
    mockedApi.getGoogleStatus.mockResolvedValue({ configured: true, connected: true, needs_reconnect: false, pending_count: 2 })
    render(<SettingsScreen />)
    expect(await screen.findByText('連携済み')).toBeInTheDocument()
    expect(screen.getByText('未同期: 2件')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '再同期' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '解除' })).toBeInTheDocument()
  })

  it('失効時は再連携の案内と「再連携」ボタンを表示する', async () => {
    mockedApi.getGoogleStatus.mockResolvedValue({
      configured: true,
      connected: false,
      needs_reconnect: true,
      pending_count: 1,
    })
    render(<SettingsScreen />)
    expect(await screen.findByRole('button', { name: '再連携' })).toBeInTheDocument()
    expect(screen.getByText(/再連携してください/)).toBeInTheDocument()
  })

  it('接続ボタンで認可URLへ遷移する', async () => {
    mockedApi.getGoogleStatus.mockResolvedValue({ configured: true, connected: false, needs_reconnect: false, pending_count: 0 })
    mockedApi.getGoogleAuthURL.mockResolvedValue({ auth_url: 'https://accounts.google.com/o/oauth2/auth?state=x' })
    // window.location.href への代入を捕捉する。
    const loc = { href: '' } as Location
    Object.defineProperty(window, 'location', { value: loc, writable: true })

    render(<SettingsScreen />)
    const btn = await screen.findByRole('button', { name: '接続' })
    fireEvent.click(btn)
    await waitFor(() => expect(loc.href).toContain('accounts.google.com'))
  })

  it('解除時に「削除する」を選択すると delete_calendar=true で呼ばれる', async () => {
    mockedApi.getGoogleStatus.mockResolvedValue({ configured: true, connected: true, needs_reconnect: false, pending_count: 0 })
    mockedApi.disconnectGoogle.mockResolvedValue(undefined)
    render(<SettingsScreen />)

    const checkbox = await screen.findByRole('checkbox')
    fireEvent.click(checkbox)
    fireEvent.click(screen.getByRole('button', { name: '解除' }))
    await waitFor(() => expect(mockedApi.disconnectGoogle).toHaveBeenCalledWith(true))
  })

  it('再同期で未同期件数が更新される', async () => {
    mockedApi.getGoogleStatus
      .mockResolvedValueOnce({ configured: true, connected: true, needs_reconnect: false, pending_count: 3 })
      .mockResolvedValueOnce({ configured: true, connected: true, needs_reconnect: false, pending_count: 0 })
    mockedApi.resyncGoogle.mockResolvedValue({ pending_count: 0 })
    render(<SettingsScreen />)

    fireEvent.click(await screen.findByRole('button', { name: '再同期' }))
    await waitFor(() => expect(screen.getByText('未同期: 0件')).toBeInTheDocument())
    expect(mockedApi.resyncGoogle).toHaveBeenCalled()
  })
})
