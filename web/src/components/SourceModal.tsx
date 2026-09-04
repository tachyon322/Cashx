import { useEffect, useState } from 'react'
import type { components } from '../api/schema'
import { useCreateSource, useUpdateSource, useOfferDomains, useRedirectPools } from '../api/queries'
import { Button } from './Button'
import { Field } from './Field'
import { Input } from './Input'
import { Modal } from './Modal'
import { Select } from './Select'
import { Textarea } from './Textarea'
import { useToast } from './Toast'
import { sourceErrorMessage } from '../lib/sourceErrors'
import { PROMO_BONUS_DEFAULT } from '../lib/affiliate'

type Source = components['schemas']['Source']
type SourceGroup = components['schemas']['SourceGroup']

interface SourceModalProps {
  open: boolean
  offerId: string
  initial: Source | null
  groups: readonly SourceGroup[]
  onClose: () => void
}

export function SourceModal({ open, offerId, initial, groups, onClose }: SourceModalProps) {
  const toast = useToast()
  const create = useCreateSource(offerId)
  const update = useUpdateSource(offerId)
  const domainsQ = useOfferDomains(offerId)
  const redirectsQ = useRedirectPools()

  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const [comment, setComment] = useState('')
  const [groupId, setGroupId] = useState('')
  const [isActive, setIsActive] = useState(true)
  const [isDefault, setIsDefault] = useState(false)
  const [type, setType] = useState<'link' | 'promo'>('link')
  const [bonus, setBonus] = useState(String(PROMO_BONUS_DEFAULT))
  const [domain, setDomain] = useState('')
  const [redirectId, setRedirectId] = useState('')

  const domains = domainsQ.data?.items ?? []
  const redirectPools = redirectsQ.data?.items ?? []
  const mainDomain = domains.find((d) => d.is_main)

  useEffect(() => {
    if (!open) return
    if (initial) {
      setName(initial.name ?? '')
      setCode(initial.code ?? '')
      setComment(initial.comment ?? '')
      setGroupId(initial.group_id ?? '')
      setIsActive(initial.is_active ?? true)
      setIsDefault(initial.is_default ?? false)
      // try to infer type from existing source: if totals has promo? fallback to link
      const t = (initial as any).type ?? 'link'
      setType(t === 'promo' ? 'promo' : 'link')
      setBonus(String((initial as any).registration_bonus ?? PROMO_BONUS_DEFAULT))
      setDomain((initial as any).domain ?? '')
      setRedirectId((initial as any).redirect_id ?? '')
    } else {
      setName('')
      setCode('')
      setComment('')
      setGroupId('')
      setIsActive(true)
      setIsDefault(false)
      setType('link')
      setBonus(String(PROMO_BONUS_DEFAULT))
      setDomain('')
      setRedirectId('')
    }
  }, [open, initial])

  const saving = create.isPending || update.isPending

  const handleSubmit = () => {
    const trimmedName = name.trim()
    if (!trimmedName) {
      toast.error('Укажите название источника')
      return
    }
    const basePayload: any = {
      name: trimmedName,
      code: code.trim() || null,
      comment: comment.trim() || null,
      group_id: groupId || null,
    }
    // Include promo-specific fields
    if (type === 'promo') {
      const nb = Number.parseInt(bonus, 10)
      if (!Number.isFinite(nb) || nb < 0) {
        toast.error('Укажите корректный бонус')
        return
      }
      basePayload.type = 'promo'
      basePayload.registration_bonus = nb
    } else {
      basePayload.type = 'link'
      if (domain) basePayload.domain = domain
      if (redirectId) basePayload.redirect_id = redirectId
    }
    if (initial?.id) {
      update.mutate(
        { ...basePayload, id: initial.id, is_active: isActive, is_default: isDefault },
        {
          onSuccess: () => {
            toast.success('Источник обновлён')
            onClose()
          },
          onError: (error) => toast.error(sourceErrorMessage(error, 'Не удалось сохранить источник')),
        },
      )
    } else {
      create.mutate(
        { ...basePayload, is_default: isDefault },
        {
          onSuccess: () => {
            toast.success('Источник создан')
            onClose()
          },
          onError: (error) => toast.error(sourceErrorMessage(error, 'Не удалось создать источник')),
        },
      )
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={initial ? 'Редактировать источник' : 'Новый источник'}>
      <div className="flex flex-col gap-4">
        <Field label="Название">
          <Input
            placeholder="Например: Telegram-канал #1"
            maxLength={60}
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </Field>

        <Field label="Код ссылки" hint="Оставьте пустым, чтобы сгенерировать автоматически. A–Z и 0–9.">
          <Input
            className="uppercase"
            placeholder="AFF2026"
            maxLength={32}
            value={code}
            onChange={(event) => setCode(event.target.value.toUpperCase())}
          />
        </Field>

        <Field label="Поток">
          <Select value={groupId} onChange={(event) => setGroupId(event.target.value)}>
            <option value="">Без потока</option>
            {groups.map((group) => (
              <option key={group.id ?? ''} value={group.id ?? ''}>
                {group.name ?? ''}
              </option>
            ))}
          </Select>
        </Field>

        <Field label="Тип">
          <Select value={type} onChange={(e) => setType(e.target.value as 'link' | 'promo')}>
            <option value="link">Ссылка</option>
            <option value="promo">Промокод</option>
          </Select>
        </Field>

        {type === 'promo' ? (
          <Field label="Бонус при регистрации, ₽" hint="Начисляется игроку по промокоду">
            <Input type="number" min="0" step="1" value={bonus} onChange={(e) => setBonus(e.target.value)} placeholder="500" />
          </Field>
        ) : (
          <>
            {domains.length > 0 && (
              <Field
                label="Домен"
                hint="Адрес ссылки и посадка клика. По умолчанию — основной домен оффера."
              >
                <Select value={domain} onChange={(e) => setDomain(e.target.value)}>
                  <option value="">
                    Авто ({mainDomain ? mainDomain.url : 'основной домен оффера'})
                  </option>
                  {domains.map((d) => (
                    <option key={d.id} value={d.url}>
                      {d.url}
                      {d.is_main ? ' — основной' : ' — запасной'}
                    </option>
                  ))}
                </Select>
              </Field>
            )}
            {redirectPools.length > 0 && (
              <Field label="Редирект (взвешенный пул)">
                <Select value={redirectId} onChange={(e) => setRedirectId(e.target.value)}>
                  <option value="">Без редиректа (прямо на оффер)</option>
                  {redirectPools.map((p: any) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </Select>
              </Field>
            )}
          </>
        )}

        <Field label="Заметка">
          <Textarea
            rows={2}
            placeholder="Заметка для себя"
            maxLength={300}
            value={comment}
            onChange={(event) => setComment(event.target.value)}
          />
        </Field>

        <label className="flex cursor-pointer items-center gap-2.5 text-[13.5px] text-text">
          <input
            type="checkbox"
            className="h-4 w-4 accent-[var(--cx-violet-bright)]"
            checked={isActive}
            onChange={(event) => setIsActive(event.target.checked)}
          />
          Источник активен
        </label>

        <label className="flex cursor-pointer items-center gap-2.5 text-[13.5px] text-text">
          <input
            type="checkbox"
            className="h-4 w-4 accent-[var(--cx-violet-bright)]"
            checked={isDefault}
            onChange={(event) => setIsDefault(event.target.checked)}
          />
          Использовать как основную ссылку оффера
        </label>

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Отмена
          </Button>
          <Button loading={saving} onClick={handleSubmit}>
            {initial ? 'Сохранить' : 'Создать'}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
