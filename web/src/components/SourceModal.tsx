import { useEffect, useState } from 'react'
import type { components } from '../api/schema'
import { useCreateSource, useUpdateSource } from '../api/queries'
import { Button } from './Button'
import { Field } from './Field'
import { Input } from './Input'
import { Modal } from './Modal'
import { Select } from './Select'
import { Textarea } from './Textarea'
import { useToast } from './Toast'
import { sourceErrorMessage } from '../lib/sourceErrors'

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

  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const [comment, setComment] = useState('')
  const [groupId, setGroupId] = useState('')
  const [isActive, setIsActive] = useState(true)
  const [isDefault, setIsDefault] = useState(false)

  useEffect(() => {
    if (!open) return
    if (initial) {
      setName(initial.name ?? '')
      setCode(initial.code ?? '')
      setComment(initial.comment ?? '')
      setGroupId(initial.group_id ?? '')
      setIsActive(initial.is_active ?? true)
      setIsDefault(initial.is_default ?? false)
    } else {
      setName('')
      setCode('')
      setComment('')
      setGroupId('')
      setIsActive(true)
      setIsDefault(false)
    }
  }, [open, initial])

  const saving = create.isPending || update.isPending

  const handleSubmit = () => {
    const trimmedName = name.trim()
    if (!trimmedName) {
      toast.error('Укажите название источника')
      return
    }
    const payload = {
      name: trimmedName,
      code: code.trim() || null,
      comment: comment.trim() || null,
      group_id: groupId || null,
    }
    if (initial?.id) {
      update.mutate(
        { ...payload, id: initial.id, is_active: isActive, is_default: isDefault },
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
        { ...payload, is_default: isDefault },
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
