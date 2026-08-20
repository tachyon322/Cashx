import { ShieldOff } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'

/** Аккаунт партнёра заблокирован администратором. */
export function BlockedPage() {
  const { user, signOut } = useAuth()
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-bg bg-hero-glow p-4">
      <div className="flex w-full max-w-[460px] flex-col items-center gap-4 rounded-lg border border-border bg-surface-1 p-4 text-center shadow-card">
        <div className="flex h-14 w-14 items-center justify-center rounded-[16px] bg-violet/14 text-lilac">
          <ShieldOff size={28} />
        </div>
        <h1 className="font-display text-[20px] font-semibold">Аккаунт заблокирован</h1>
        <p className="text-[14px] text-muted">
          {user?.name ? `${user.name}, ваш` : 'Ваш'} аккаунт заблокирован администратором
          платформы. Доступ к кабинету партнёра приостановлен.
        </p>
        <div className="flex flex-col gap-1 font-semibold">
          <span>{user?.name}</span>
          <span className="text-faint">{user?.email}</span>
        </div>
        <p className="text-[14px] text-faint">
          Если вы считаете, что это ошибка, обратитесь в поддержку:{' '}
          <a href="mailto:support@cashx.local">support@cashx.local</a>
        </p>
        <div className="mt-1.5 flex gap-3">
          <button
            type="button"
            className="inline-flex cursor-pointer items-center justify-center rounded-md border border-border bg-transparent px-5 py-[9px] text-[13.5px] font-medium text-text transition-colors duration-150 hover:bg-surface-hover"
            onClick={() => void signOut()}
          >
            Выйти
          </button>
        </div>
      </div>
    </div>
  )
}