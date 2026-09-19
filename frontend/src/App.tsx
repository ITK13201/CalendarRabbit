import { useState } from 'react'
import { BottomTabs, type TabKey } from './components/BottomTabs'
import { ChatScreen } from './screens/ChatScreen'
import { CalendarScreen } from './screens/CalendarScreen'
import { SettingsScreen } from './screens/SettingsScreen'

export function App() {
  // 初期表示はカレンダー画面（spec: pwa-web-app 初期表示）。
  const [tab, setTab] = useState<TabKey>('calendar')

  return (
    <div className="app">
      <main className="app-content">
        {tab === 'chat' && <ChatScreen />}
        {tab === 'calendar' && <CalendarScreen />}
        {tab === 'settings' && <SettingsScreen />}
      </main>
      <BottomTabs active={tab} onChange={setTab} />
    </div>
  )
}
