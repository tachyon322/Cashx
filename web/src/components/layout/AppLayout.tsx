import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { Topbar } from './Topbar'

/** Каркас приложения: сайдбар (fixed 248px) + топбар (sticky 64px) + контент. */
export function AppLayout() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const closeDrawer = () => setDrawerOpen(false)

  return (
    <div className="min-h-screen">
      <Sidebar open={drawerOpen} onClose={closeDrawer} />
      {drawerOpen && (
        <div
          className="fixed inset-0 z-[55] hidden bg-[rgba(5,5,13,0.65)] max-[899px]:block"
          onClick={closeDrawer}
        />
      )}
      <div className="ml-[248px] flex min-h-screen flex-col max-[899px]:ml-0">
        <Topbar onMenu={() => setDrawerOpen(true)} />
        <main className="flex w-full flex-col gap-4 p-4">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

/** Кабинет партнёра использует тот же каркас. */
export function PartnerLayout() {
  return <AppLayout />
}