import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Plus, Search, ShieldCheck } from 'lucide-react'
import { useAdminStaff, useAdminStaffCreate, useAdminStaffUpdate, type StaffMember } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Modal } from '../../components/Modal'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { useToast } from '../../components/Toast'
import { formatDate } from '../../lib/format'

const ALL_ROLES: { value: string; label: string }[] = [
  { value: 'superadmin', label: 'Супер-админ' },
  { value: 'project_manager', label: 'Project Manager' },
  { value: 'finance', label: 'Finance' },
  { value: 'content_manager', label: 'Content' },
  { value: 'support', label: 'Support' },
]

function roleBadge(role: string) {
  const tone = role === 'superadmin' ? 'success' : role === 'finance' ? 'warning' : 'neutral'
  const label = ALL_ROLES.find((r) => r.value === role)?.label ?? role
  return (
    <Badge key={role} tone={tone as any}>
      {label}
    </Badge>
  )
}

export function StaffPage() {
  const toast = useToast()
  const [search, setSearch] = useState('')
  const [applied, setApplied] = useState('')
  useEffect(() => {
    const t = window.setTimeout(() => setApplied(search.trim()), 300)
    return () => window.clearTimeout(t)
  }, [search])

  const q = useAdminStaff(applied)
  const items = q.data?.items ?? []
  const total = q.data?.total ?? 0

  // create modal
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [roles, setRoles] = useState<string[]>(['superadmin'])
  const createMut = useAdminStaffCreate()

  const toggleRole = (r: string) => {
    setRoles((prev) => (prev.includes(r) ? prev.filter((x) => x !== r) : [...prev, r]))
  }

  const submitCreate = async (e: FormEvent) => {
    e.preventDefault()
    if (roles.length === 0) {
      toast.error('Выберите хотя бы одну роль')
      return
    }
    try {
      await createMut.mutateAsync({ name: name.trim(), email: email.trim(), password, roles })
      toast.success('Сотрудник создан')
      setCreateOpen(false)
      setName('')
      setEmail('')
      setPassword('')
      setRoles(['superadmin'])
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Не удалось создать')
    }
  }

  // edit modal
  const [edit, setEdit] = useState<StaffMember | null>(null)
  const [editRoles, setEditRoles] = useState<string[]>([])
  const [editName, setEditName] = useState('')
  const [editEmail, setEditEmail] = useState('')
  const [editPassword, setEditPassword] = useState('')
  const [editActive, setEditActive] = useState(true)
  const updateMut = useAdminStaffUpdate()

  const openEdit = (m: StaffMember) => {
    setEdit(m)
    setEditRoles(m.roles)
    setEditName(m.name)
    setEditEmail(m.email)
    setEditActive(m.is_active)
    setEditPassword('')
  }

  const toggleEditRole = (r: string) => {
    setEditRoles((prev) => (prev.includes(r) ? prev.filter((x) => x !== r) : [...prev, r]))
  }

  const submitEdit = async (e: FormEvent) => {
    e.preventDefault()
    if (!edit) return
    if (editRoles.length === 0) {
      toast.error('Выберите хотя бы одну роль')
      return
    }
    try {
      await updateMut.mutateAsync({
        id: edit.id,
        name: editName.trim(),
        email: editEmail.trim(),
        is_active: editActive,
        roles: editRoles,
        ...(editPassword ? { password: editPassword } : {}),
      })
      toast.success('Сохранено')
      setEdit(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Не удалось сохранить')
    }
  }

  return (
    <div className="flex w-full flex-col gap-4 p-4">
      <Card
        title="Команда"
        subtitle={`Всего: ${total} · доступно только суперадмину`}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus size={16} />
            Добавить сотрудника
          </Button>
        }
      >
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-[240px] max-w-[420px] flex-1">
            <Field label="Поиск">
              <div className="relative">
                <Search size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
                <Input
                  type="search"
                  placeholder="Имя или email"
                  className="pl-[34px]"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
            </Field>
          </div>
        </div>

        {q.isLoading && items.length === 0 ? (
          <div className="flex flex-col gap-2">
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              { key: 'name', header: 'Имя', render: (m: StaffMember) => m.name ?? '—' },
              { key: 'email', header: 'Email', render: (m: StaffMember) => m.email ?? '—' },
              {
                key: 'roles',
                header: 'Роли',
                render: (m: StaffMember) => (
                  <span className="flex flex-wrap gap-1">
                    {m.roles.length ? m.roles.map(roleBadge) : <span className="text-faint">—</span>}
                  </span>
                ),
              },
              {
                key: 'active',
                header: 'Статус',
                render: (m: StaffMember) =>
                  m.is_active ? <Badge tone="success">Активен</Badge> : <Badge tone="danger">Отключён</Badge>,
              },
              {
                key: 'created_at',
                header: 'Создан',
                render: (m: StaffMember) => (m.created_at ? formatDate(m.created_at) : '—'),
              },
            ]}
            rows={items}
            rowKey={(m: StaffMember) => m.id}
            onRowClick={(m: StaffMember) => openEdit(m)}
            emptyTitle="Сотрудники не найдены"
            emptyHint="Добавьте первого сотрудника"
          />
        )}
      </Card>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Добавить сотрудника">
        <form className="flex flex-col gap-4" onSubmit={submitCreate}>
          <Field label="Имя" htmlFor="st-name">
            <Input id="st-name" required autoFocus value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label="Email" hint="Если email уже есть у партнёра — он будет повышен до сотрудника" htmlFor="st-email">
            <Input id="st-email" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <Field label="Пароль" htmlFor="st-pass">
            <Input id="st-pass" type="password" required minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} />
          </Field>
          <Field label="Роли">
            <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface-1 p-3">
              {ALL_ROLES.map((r) => (
                <label key={r.value} className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={roles.includes(r.value)} onChange={() => toggleRole(r.value)} />
                  <span>{r.label}</span>
                  {r.value === 'superadmin' && <ShieldCheck size={14} className="text-violet-bright" />}
                </label>
              ))}
            </div>
          </Field>
          <div className="mt-1 flex justify-end gap-3">
            <Button variant="secondary" onClick={() => setCreateOpen(false)} type="button">
              Отмена
            </Button>
            <Button type="submit" loading={createMut.isPending}>
              Создать
            </Button>
          </div>
        </form>
      </Modal>

      <Modal open={!!edit} onClose={() => setEdit(null)} title={edit ? `Редактировать: ${edit.email}` : ''}>
        {edit && (
          <form className="flex flex-col gap-4" onSubmit={submitEdit}>
            <Field label="Имя" htmlFor="ed-name">
              <Input id="ed-name" value={editName} onChange={(e) => setEditName(e.target.value)} />
            </Field>
            <Field label="Email" htmlFor="ed-email">
              <Input id="ed-email" type="email" value={editEmail} onChange={(e) => setEditEmail(e.target.value)} />
            </Field>
            <Field label="Новый пароль" hint="Оставьте пустым чтобы не менять" htmlFor="ed-pass">
              <Input id="ed-pass" type="password" minLength={8} value={editPassword} onChange={(e) => setEditPassword(e.target.value)} />
            </Field>
            <Field label="Активен">
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={editActive} onChange={(e) => setEditActive(e.target.checked)} />
                Аккаунт активен
              </label>
            </Field>
            <Field label="Роли">
              <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface-1 p-3">
                {ALL_ROLES.map((r) => (
                  <label key={r.value} className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={editRoles.includes(r.value)} onChange={() => toggleEditRole(r.value)} />
                    <span>{r.label}</span>
                    {r.value === 'superadmin' && <ShieldCheck size={14} className="text-violet-bright" />}
                  </label>
                ))}
              </div>
            </Field>
            <div className="mt-1 flex justify-end gap-3">
              <Button variant="secondary" onClick={() => setEdit(null)} type="button">
                Отмена
              </Button>
              <Button type="submit" loading={updateMut.isPending}>
                Сохранить
              </Button>
            </div>
          </form>
        )}
      </Modal>
    </div>
  )
}
