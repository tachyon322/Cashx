import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { ImagePlus, Save } from 'lucide-react'
import { useAdminBranding, useAdminBrandingPut, useAdminMediaUpload } from '../../api/queries'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Skeleton } from '../../components/Skeleton'
import { useToast } from '../../components/Toast'

const MAX_AVATAR_BYTES = 10 * 1024 * 1024

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex max-w-[560px] flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function BrandingPage() {
  const toast = useToast()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const branding = useAdminBranding()
  const putMutation = useAdminBrandingPut()
  const uploadMutation = useAdminMediaUpload()

  const [name, setName] = useState('')
  const [telegramUrl, setTelegramUrl] = useState('')
  const [avatarMediaId, setAvatarMediaId] = useState('')
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null)

  // Первичное заполнение формы из текущего брендинга.
  useEffect(() => {
    if (branding.data) {
      setName(branding.data.name ?? '')
      setTelegramUrl(branding.data.telegram_url ?? '')
      if (avatarPreview === null) setAvatarPreview(branding.data.avatar_url ?? null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [branding.data])

  const pickFile = (file: File | undefined) => {
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error('Можно загружать только изображения')
      return
    }
    if (file.size > MAX_AVATAR_BYTES) {
      toast.error('Файл слишком большой — максимум 10 МБ')
      return
    }
    uploadMutation.mutate(file, {
      onSuccess: (result) => {
        setAvatarMediaId(result.media_id)
        setAvatarPreview(result.url)
        toast.success('Аватар загружен')
      },
      onError: (error) => {
        toast.error(error instanceof Error ? error.message : 'Не удалось загрузить файл')
      },
    })
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      await putMutation.mutateAsync({
        name: name.trim() || undefined,
        telegram_url: telegramUrl.trim() || undefined,
        ...(avatarMediaId ? { avatar_media_id: avatarMediaId } : {}),
      })
      toast.success('Брендинг сохранён')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить брендинг')
    }
  }

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
                {avatarPreview ? (
                  <img className="flex h-24 w-24 rounded-lg border border-border bg-surface-1 object-cover" src={avatarPreview} alt="Аватар платформы" />
                ) : (
                  <div className="flex h-24 w-24 items-center justify-center rounded-lg border border-border bg-surface-1 text-faint">
                    <ImagePlus size={28} />
                  </div>
                )}
              </Field>
              <div className="flex flex-col items-start gap-2 pb-0.5">
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  className="sr-only"
                  onChange={(event) => {
                    pickFile(event.target.files?.[0])
                    event.target.value = ''
                  }}
                />
                <Button
                  variant="secondary"
                  loading={uploadMutation.isPending}
                  onClick={() => fileInputRef.current?.click()}
                >
                  <ImagePlus size={16} />
                  Загрузить аватар
                </Button>
                {avatarMediaId && (
                  <span className="text-[12.5px] text-faint">
                    Новый аватар сохранён в форме
                  </span>
                )}
                <span className="text-[12.5px] text-faint">
                  PNG/JPG/WebP, до 10 МБ
                </span>
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