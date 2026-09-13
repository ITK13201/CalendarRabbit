export type TabKey = 'chat' | 'calendar' | 'settings'

interface BottomTabsProps {
  active: TabKey
  onChange: (tab: TabKey) => void
}

const TABS: { key: TabKey; label: string; icon: string }[] = [
  { key: 'chat', label: 'チャット', icon: '💬' },
  { key: 'calendar', label: 'カレンダー', icon: '📅' },
  { key: 'settings', label: '設定', icon: '⚙️' },
]

export function BottomTabs({ active, onChange }: BottomTabsProps) {
  return (
    <nav className="bottom-tabs" aria-label="メインナビゲーション">
      {TABS.map((t) => (
        <button
          key={t.key}
          type="button"
          className={`tab ${active === t.key ? 'tab-active' : ''}`}
          aria-current={active === t.key ? 'page' : undefined}
          onClick={() => onChange(t.key)}
        >
          <span className="tab-icon" aria-hidden="true">
            {t.icon}
          </span>
          <span className="tab-label">{t.label}</span>
        </button>
      ))}
    </nav>
  )
}
