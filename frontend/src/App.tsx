import { useState } from 'react'
import { BottomTabs, type TabKey } from './components/BottomTabs'
import { ChatScreen } from './screens/ChatScreen'
import { CalendarScreen } from './screens/CalendarScreen'
import { SettingsScreen } from './screens/SettingsScreen'

// initialTab は Google 連携コールバックからの復帰（?google=...）時は設定画面を初期表示する。
function initialTab(): TabKey {
  if (typeof window !== 'undefined' && new URLSearchParams(window.location.search).has('google')) {
    return 'settings'
  }
  return 'calendar'
}

export function App() {
  // 初期表示はカレンダー画面（spec: pwa-web-app 初期表示）。
  const [tab, setTab] = useState<TabKey>(initialTab)

  return (
    <div className="flex h-[100dvh] flex-col">
      <main className="flex flex-1 overflow-hidden">
        {tab === 'chat' && <ChatScreen />}
        {tab === 'calendar' && <CalendarScreen />}
        {tab === 'settings' && <SettingsScreen />}
      </main>
      <BottomTabs active={tab} onChange={setTab} />
    </div>
  )
}
