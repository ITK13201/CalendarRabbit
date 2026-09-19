export type TabKey = 'chat' | 'calendar' | 'settings'

interface BottomTabsProps {
  active: TabKey
  onChange: (tab: TabKey) => void
}

const TABS: { key: TabKey; label: string; icon: string }[] = [
  { key: 'calendar', label: 'カレンダー', icon: '📅' },
  { key: 'chat', label: 'チャット', icon: '💬' },
  { key: 'settings', label: '設定', icon: '⚙️' },
]

export function BottomTabs({ active, onChange }: BottomTabsProps) {
  return (
    <nav
      className="bottom-tabs flex border-t border-border bg-white/90 backdrop-blur-md backdrop-saturate-[1.8]"
      aria-label="メインナビゲーション"
    >
      {TABS.map((t) => {
        const isActive = active === t.key
        return (
          <button
            key={t.key}
            type="button"
            className={`relative flex flex-1 cursor-pointer flex-col items-center justify-center gap-[3px] border-none bg-none text-[0.7rem] transition-colors [-webkit-tap-highlight-color:transparent] ${
              isActive ? 'font-semibold text-primary' : 'font-medium text-muted hover:text-text'
            }`}
            aria-current={isActive ? 'page' : undefined}
            onClick={() => onChange(t.key)}
          >
            {isActive && (
              <span
                className="absolute top-0 h-[3px] w-7 rounded-full bg-primary"
                aria-hidden="true"
              />
            )}
            <span
              className={`text-[1.3rem] leading-none transition-transform ${
                isActive ? '-translate-y-px scale-105' : ''
              }`}
              aria-hidden="true"
            >
              {t.icon}
            </span>
            <span>{t.label}</span>
          </button>
        )
      })}
    </nav>
  )
}
