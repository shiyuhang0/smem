import { DashboardPage } from '../pages/dashboard-page'
import { Providers } from './providers'

export function App() {
  return (
    <Providers>
      <DashboardPage />
    </Providers>
  )
}
