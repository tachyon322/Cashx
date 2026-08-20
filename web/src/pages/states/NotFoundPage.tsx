import { SearchX } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../auth/AuthContext'

/** 404. */
export function NotFoundPage() {
  const { user } = useAuth()
  const home = user ? (user.role === 'staff' ? '/admin/partners' : '/cabinet') : '/login'
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-bg bg-hero-glow p-4">
      <div className="flex w-full max-w-[460px] flex-col items-center gap-4 rounded-lg border border-border bg-surface-1 p-4 text-center shadow-card">
        <div className="flex h-14 w-14 items-center justify-center rounded-[16px] bg-violet/14 text-lilac">
          <SearchX size={28} />
        </div>
        <h1 className="font-display text-[20px] font-semibold">Страница не найдена</h1>
        <p className="text-[14px] text-muted">
          Такой страницы не существует — возможно, ссылка устарела или адрес введён неверно.
        </p>
        <div className="mt-1.5 flex gap-3">
          <Link
            to={home}
            className="inline-flex items-center justify-center rounded-md border border-border bg-transparent px-5 py-[9px] text-[13.5px] font-medium text-text transition-colors duration-150 hover:bg-surface-hover"
          >
            На главную
          </Link>
        </div>
      </div>
    </div>
  )
}