import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { ImagePlus, Save } from 'lucide-react'
import { useAdminBranding, useAdminBrandingPut } from '../../api/queries'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Skeleton } from '../../components/Skeleton'
import { useToast } from '../../components/Toast'

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex max-w-[560px] flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function BrandingPage() {
  const toast = useToast()

  const branding = useAdminBranding()
  const putMutation = useAdminBrandingPut()

  const [name, setName] = useState('')
  const [telegramUrl, setTelegramUrl] = useState('')
  const [avatarUrl, setAvatarUrl] = useState('')

  // Первичное заполнение формы из текущего брендинга.
  useEffect(() => {
    if (branding.data) {
      setName(branding.data.name ?? '')
      setTelegramUrl(branding.data.telegram_url ?? '')
      setAvatarUrl(branding.data.avatar_url ?? '')
    }
  }, [branding.data])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      await putMutation.mutateAsync({
        name: name.trim() || undefined,
        telegram_url: telegramUrl.trim() || undefined,
        avatar_url: avatarUrl.trim() ? avatarUrl.trim() : null,
      })
      toast.success('Брендинг сохранён')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить брендинг')
    }
  }

  const preview = avatarUrl.trim() || branding.data?.avatar_url?.trim() || null

  return (
    <div className={PAGE_CLASSES}>
      <Card title="Брендинг платформы" subtitle="Название, Telegram и аватар, видимые партнёрам">
        {branding.isLoading ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 96 }} />
            <Skeleton style={{ height: 40 }} />
            <Skeleton style={{ height: 40 }} />
          </div>
        ) : (
          <form className={MODAL_FORM_CLASSES} onSubmit={submit}>
            <div className="flex flex-wrap items-end gap-4">
              <Field label="Аватар">
                {preview ? (
                  <img className="flex h-24 w-24 rounded-lg border border-border bg-surface-1 object-cover" src={preview} alt="Аватар платформы" />
                ) : (
                  <div className="flex h-24 w-24 items-center justify-center rounded-lg border border-border bg-surface-1 text-faint">
                    <ImagePlus size={28} />
                  </div>
                )}
              </Field>
              <div className="flex flex-1 flex-col gap-2 pb-0.5">
                <Field label="URL аватара" htmlFor="br-avatar">
                  <Input
                    id="br-avatar"
                    type="url"
                    placeholder="https://example.com/avatar.png"
                    value={avatarUrl}
                    onChange={(event) => setAvatarUrl(event.target.value)}
                  />
                </Field>
                <span className="text-[12.5px] text-faint">Прямая ссылка на изображение (https)</span>
              </div>
            </div>
            <Field label="Название платформы" htmlFor="br-name">
              <Input
                id="br-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <Field label="Telegram-ссылка" htmlFor="br-tg">
              <Input
                id="br-tg"
                type="url"
                placeholder="https://t.me/..."
                value={telegramUrl}
                onChange={(event) => setTelegramUrl(event.target.value)}
              />
            </Field>
            <div className={MODAL_ACTIONS_CLASSES}>
              <Button type="submit" loading={putMutation.isPending}>
                <Save size={16} />
                Сохранить
              </Button>
            </div>
          </form>
        )}
      </Card>
    </div>
  )
}