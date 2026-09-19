import { useState } from 'react'
import { BottomTabs, type TabKey } from './components/BottomTabs'
import { ChatScreen } from './screens/ChatScreen'
import { CalendarScreen } from './screens/CalendarScreen'
import { SettingsScreen } from './screens/SettingsScreen'

export function App() {
  // 初期表示はカレンダー画面（spec: pwa-web-app 初期表示）。
  const [tab, setTab] = useState<TabKey>('calendar')

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
