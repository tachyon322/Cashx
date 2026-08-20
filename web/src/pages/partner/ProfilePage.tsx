import { useState } from 'react'
import type { FormEvent } from 'react'
import { useAuth } from '../../auth/AuthContext'
import { useSummary, useUpdateProfile } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Skeleton } from '../../components/Skeleton'
import { useToast } from '../../components/Toast'

export function ProfilePage() {
  const { user, signOut } = useAuth()
  const toast = useToast()
  const { data: summary, isLoading: summaryLoading } = useSummary()
  const update = useUpdateProfile()

  const [name, setName] = useState(user?.name ?? '')
  const [telegramId, setTelegramId] = useState(
    user?.partner?.telegram_user_id != null ? String(user.partner.telegram_user_id) : '',
  )
  const [telegramError, setTelegramError] = useState<string | null>(null)

  const connected = summary?.telegram?.connected === true
  const currentId = user?.partner?.telegram_user_id

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    setTelegramError(null)

    const trimmed = name.trim()
    const tg = telegramId.trim()

    if (tg !== '' && !/^\d+$/.test(tg)) {
      setTelegramError('Telegram ID — это число, например 123456789')
      return
    }

    update.mutate(
      {
        name: trimmed,
        ...(tg !== '' ? { telegram_user_id: Number(tg) } : {}),
      },
      {
        onSuccess: () => toast.success('Профиль сохранён'),
        onError: (error) =>
          toast.error(error instanceof Error ? error.message : 'Не удалось сохранить профиль'),
      },
    )
  }

  if (summaryLoading) {
    return (
      <div className="max-w-[560px]">
        <Skeleton style={{ height: 380 }} />
      </div>
    )
  }

  return (
    <div className="max-w-[560px]">
      <Card
        title="Профиль"
        subtitle="Основные данные и уведомления"
        actions={
          <Button variant="danger" onClick={() => void signOut()}>
            Выйти
          </Button>
        }
      >
        <form className="flex flex-col gap-4" onSubmit={handleSubmit} noValidate>
          <Field label="Имя" htmlFor="profile-name">
            <Input
              id="profile-name"
              value={name}
              placeholder="Ваше имя"
              onChange={(event) => setName(event.target.value)}
            />
          </Field>
          <Field
            label="Telegram ID"
            htmlFor="profile-telegram"
            error={telegramError}
            hint={
              connected
                ? undefined
                : 'Напишите @userinfobot — бот пришлёт ваш числовой id'
            }
          >
            <div className="flex flex-wrap items-center gap-3">
              <Input
                id="profile-telegram"
                type="number"
                inputMode="numeric"
                placeholder="Например, 123456789"
                className="min-w-[180px] flex-1"
                value={telegramId}
                onChange={(event) => setTelegramId(event.target.value)}
              />
              {connected && (
                <Badge tone="success">
                  Подключен{currentId != null ? ` · ${currentId}` : ''}
                </Badge>
              )}
            </div>
          </Field>
          <div className="mt-1">
            <Button type="submit" loading={update.isPending}>
              Сохранить
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}