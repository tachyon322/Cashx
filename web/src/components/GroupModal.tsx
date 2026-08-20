import { useEffect, useState } from 'react'
import type { components } from '../api/schema'
import { useCreateGroup, useUpdateGroup } from '../api/queries'
import { Button } from './Button'
import { Field } from './Field'
import { Input } from './Input'
import { Modal } from './Modal'
import { Textarea } from './Textarea'
import { useToast } from './Toast'
import { sourceErrorMessage } from '../lib/sourceErrors'

type SourceGroup = components['schemas']['SourceGroup']

interface GroupModalProps {
  open: boolean
  initial: SourceGroup | null
  onClose: () => void
}

export function GroupModal({ open, initial, onClose }: GroupModalProps) {
  const toast = useToast()
  const create = useCreateGroup()
  const update = useUpdateGroup()

  const [name, setName] = useState('')
  const [comment, setComment] = useState('')

  useEffect(() => {
    if (!open) return
    setName(initial?.name ?? '')
    setComment(initial?.comment ?? '')
  }, [open, initial])

  const saving = create.isPending || update.isPending

  const handleSubmit = () => {
    const trimmedName = name.trim()
    if (!trimmedName) {
      toast.error('Укажите название потока')
      return
    }
    const payload = { name: trimmedName, comment: comment.trim() || null }
    if (initial?.id) {
      update.mutate(
        { ...payload, id: initial.id },
        {
          onSuccess: () => {
            toast.success('Поток обновлён')
            onClose()
          },
          onError: (error) => toast.error(sourceErrorMessage(error, 'Не удалось сохранить поток')),
        },
      )
    } else {
      create.mutate(payload, {
        onSuccess: () => {
          toast.success('Поток создан')
          onClose()
        },
        onError: (error) => toast.error(sourceErrorMessage(error, 'Не удалось создать поток')),
      })
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={initial ? 'Редактировать поток' : 'Новый поток'}>
      <div className="flex flex-col gap-4">
        <Field label="Название">
          <Input
            placeholder="Например: Telegram"
            maxLength={60}
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
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
