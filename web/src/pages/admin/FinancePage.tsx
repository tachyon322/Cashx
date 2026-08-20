import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Save } from 'lucide-react'
import {
  useAdminFinanceEarnings,
  useAdminFinanceLedger,
  useAdminFinanceRules,
  useAdminFinanceRulesPut,
  useAdminOffers,
  useAdminPartners,
} from '../../api/queries'
import type { AdminEarningsParams, AdminListParams } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { PAGE_SIZE, Pagination } from '../../components/Pagination'
import { Select } from '../../components/Select'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { useToast } from '../../components/Toast'
import { formatBps, formatDateTime, formatRubles } from '../../lib/format'
import { cx } from '../../lib/cx'

type FinanceTab = 'rules' | 'ledger' | 'earnings'

const TABS: { value: FinanceTab; label: string }[] = [
  { value: 'rules', label: 'Правила' },
  { value: 'ledger', label: 'Ledger' },
  { value: 'earnings', label: 'Начисления' },
]

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const TABS_CLASSES = 'flex w-fit max-w-full flex-wrap gap-1 rounded-md border border-border bg-surface-0 p-1'
const TAB_BTN_CLASSES =
  'cursor-pointer rounded-md border-none bg-transparent px-3.5 py-[7px] text-[13px] font-semibold text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text'
const TAB_BTN_ACTIVE_CLASSES = 'bg-surface-2 text-text shadow-[0_0_0_1px_var(--cx-border-active)]'
const TOOLBAR_CLASSES = 'flex flex-wrap items-end gap-3'
const TOOLBAR_FIELD_CLASSES = 'min-w-[180px] max-w-[320px] flex-1'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ROW_CLASSES = 'grid grid-cols-2 gap-3 max-[560px]:grid-cols-1'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function FinancePage() {
  const toast = useToast()
  const [tab, setTab] = useState<FinanceTab>('rules')

  /* --- Вкладка «Правила» --- */
  const rules = useAdminFinanceRules()
  const rulesPut = useAdminFinanceRulesPut()

  const [minWithdraw, setMinWithdraw] = useState('')
  const [usdtRate, setUsdtRate] = useState('')
  const [sbpFlat, setSbpFlat] = useState('')
  const [sbpPercent, setSbpPercent] = useState('')

  useEffect(() => {
    if (rules.data) {
      setMinWithdraw(String(rules.data.min_withdraw_kopecks ?? ''))
      setUsdtRate(String(rules.data.usdt_rate ?? ''))
      setSbpFlat(String(rules.data.sbp_fee_flat_kopecks ?? ''))
      setSbpPercent(String(rules.data.sbp_fee_percent_bps ?? ''))
    }
  }, [rules.data])

  const submitRules = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const result = await rulesPut.mutateAsync({
        min_withdraw_kopecks: minWithdraw ? Number(minWithdraw) : undefined,
        usdt_rate: usdtRate ? Number(usdtRate) : undefined,
        sbp_fee_flat_kopecks: sbpFlat ? Number(sbpFlat) : undefined,
        sbp_fee_percent_bps: sbpPercent ? Number(sbpPercent) : undefined,
      })
      if (result) {
        setMinWithdraw(String(result.min_withdraw_kopecks ?? ''))
        setUsdtRate(String(result.usdt_rate ?? ''))
        setSbpFlat(String(result.sbp_fee_flat_kopecks ?? ''))
        setSbpPercent(String(result.sbp_fee_percent_bps ?? ''))
      }
      toast.success('Правила выплат сохранены')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить правила')
    }
  }

  /* --- Вкладка «Ledger» --- */
  const [ledgerPartner, setLedgerPartner] = useState('')
  const [ledgerFrom, setLedgerFrom] = useState('')
  const [ledgerTo, setLedgerTo] = useState('')
  const [ledgerOffset, setLedgerOffset] = useState(0)

  useEffect(() => {
    setLedgerOffset(0)
  }, [ledgerPartner, ledgerFrom, ledgerTo])

  const ledgerParams: AdminListParams = {
    limit: PAGE_SIZE,
    offset: ledgerOffset,
    ...(ledgerPartner ? { partner_id: ledgerPartner } : {}),
    ...(ledgerFrom ? { from: ledgerFrom } : {}),
    ...(ledgerTo ? { to: ledgerTo } : {}),
  }
  const ledger = useAdminFinanceLedger(ledgerParams)
  const ledgerItems = ledger.data?.items ?? []

  /* --- Вкладка «Начисления» --- */
  const [earnPartner, setEarnPartner] = useState('')
  const [earnOffer, setEarnOffer] = useState('')
  const [earnFrom, setEarnFrom] = useState('')
  const [earnTo, setEarnTo] = useState('')
  const [earnOffset, setEarnOffset] = useState(0)

  useEffect(() => {
    setEarnOffset(0)
  }, [earnPartner, earnOffer, earnFrom, earnTo])

  const earningsParams: AdminEarningsParams = {
    limit: PAGE_SIZE,
    offset: earnOffset,
    ...(earnPartner ? { partner_id: earnPartner } : {}),
    ...(earnOffer ? { offer_id: earnOffer } : {}),
    ...(earnFrom ? { from: earnFrom } : {}),
    ...(earnTo ? { to: earnTo } : {}),
  }
  const earnings = useAdminFinanceEarnings(earningsParams)
  const earningsItems = earnings.data?.items ?? []

  // Справочники для имён (в ответе начислений только id).
  const partners = useAdminPartners({ limit: 200 })
  const offers = useAdminOffers()
  const partnerName = (id?: string) => partners.data?.items?.find((p) => p.id === id)?.name
  const offerName = (id?: string) => offers.data?.items?.find((o) => o.id === id)?.name

  return (
    <div className={PAGE_CLASSES}>
      <div className={TABS_CLASSES} role="tablist">
        {TABS.map((item) => (
          <button
            key={item.value}
            type="button"
            role="tab"
            aria-selected={tab === item.value}
            className={cx(TAB_BTN_CLASSES, tab === item.value && TAB_BTN_ACTIVE_CLASSES)}
            onClick={() => setTab(item.value)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {tab === 'rules' && (
        <Card
          title="Правила выплат"
          subtitle="Лимиты, курс и комиссии, применяемые к заявкам партнёров"
        >
          {rules.isLoading ? (
            <div className={SKELETON_CLASSES}>
              <Skeleton style={{ height: 40 }} />
              <Skeleton style={{ height: 40 }} />
              <Skeleton style={{ height: 40 }} />
            </div>
          ) : (
            <form className={MODAL_FORM_CLASSES} onSubmit={submitRules}>
              <div className={MODAL_ROW_CLASSES}>
                <Field label="Мин. сумма вывода, копеек" htmlFor="fi-min">
                  <Input
                    id="fi-min"
                    type="number"
                    min={0}
                    step={1}
                    value={minWithdraw}
                    onChange={(event) => setMinWithdraw(event.target.value)}
                  />
                </Field>
                <Field label="Курс USDT" htmlFor="fi-rate">
                  <Input
                    id="fi-rate"
                    type="number"
                    min={0}
                    step="any"
                    value={usdtRate}
                    onChange={(event) => setUsdtRate(event.target.value)}
                  />
                </Field>
              </div>
              <div className={MODAL_ROW_CLASSES}>
                <Field label="Комиссия СБП (фикс.), копеек" htmlFor="fi-flat">
                  <Input
                    id="fi-flat"
                    type="number"
                    min={0}
                    step={1}
                    value={sbpFlat}
                    onChange={(event) => setSbpFlat(event.target.value)}
                  />
                </Field>
                <Field label="Комиссия СБП (%), бпс" htmlFor="fi-percent">
                  <Input
                    id="fi-percent"
                    type="number"
                    min={0}
                    step={1}
                    value={sbpPercent}
                    onChange={(event) => setSbpPercent(event.target.value)}
                  />
                </Field>
              </div>
              <div className={MODAL_ACTIONS_CLASSES}>
                <Button type="submit" loading={rulesPut.isPending}>
                  <Save size={16} />
                  Сохранить
                </Button>
              </div>
            </form>
          )}

          {rules.data && (
            <div className="mt-1 flex flex-col gap-2 rounded-md border border-border bg-surface-0 p-4">
              <div className="text-[10.5px] font-semibold uppercase leading-[1.4] tracking-[0.08em] text-muted">
                Текущие правила
              </div>
              <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-x-6 gap-y-2 text-[13.5px]">
                <div>
                  <span className="text-faint">Мин. сумма:</span>{' '}
                  {formatRubles(rules.data.min_withdraw_kopecks ?? 0)}
                </div>
                <div>
                  <span className="text-faint">Курс USDT:</span> {rules.data.usdt_rate ?? '—'}
                </div>
                <div>
                  <span className="text-faint">Комиссия СБП (фикс.):</span>{' '}
                  {formatRubles(rules.data.sbp_fee_flat_kopecks ?? 0)}
                </div>
                <div>
                  <span className="text-faint">Комиссия СБП (%):</span>{' '}
                  {formatBps(rules.data.sbp_fee_percent_bps ?? 0)}
                </div>
              </div>
              {rules.data.updated_at && (
                <div className="text-[12.5px] text-faint">
                  Обновлено: {formatDateTime(rules.data.updated_at)}
                </div>
              )}
            </div>
          )}
        </Card>
      )}

      {tab === 'ledger' && (
        <Card title="Ledger" subtitle={`Всего операций: ${ledger.data?.total ?? 0}`}>
          <div className={TOOLBAR_CLASSES}>
            <Field label="Партнёр" className={TOOLBAR_FIELD_CLASSES}>
              <Select value={ledgerPartner} onChange={(event) => setLedgerPartner(event.target.value)}>
                <option value="">Все партнёры</option>
                {partners.data?.items?.map((partner) => (
                  <option key={partner.id} value={partner.id}>
                    {partner.name ?? partner.email ?? partner.id}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="С даты" className={TOOLBAR_FIELD_CLASSES}>
              <Input
                type="date"
                value={ledgerFrom}
                onChange={(event) => setLedgerFrom(event.target.value)}
              />
            </Field>
            <Field label="По дату" className={TOOLBAR_FIELD_CLASSES}>
              <Input
                type="date"
                value={ledgerTo}
                onChange={(event) => setLedgerTo(event.target.value)}
              />
            </Field>
          </div>

          {ledger.isLoading && ledgerItems.length === 0 ? (
            <div className={SKELETON_CLASSES}>
              <Skeleton style={{ height: 36 }} />
              <Skeleton style={{ height: 36 }} />
              <Skeleton style={{ height: 36 }} />
            </div>
          ) : (
            <Table
              columns={[
                { key: 'partner_name', header: 'Партнёр', render: (l) => l.partner_name ?? '—' },
                { key: 'type', header: 'Тип', render: (l) => l.type ?? '—' },
                {
                  key: 'amount',
                  header: 'Сумма',
                  align: 'right',
                  render: (l) => (
                    <span className="tabular-nums">{formatRubles(l.amount_kopecks ?? 0)}</span>
                  ),
                },
                {
                  key: 'balance_after',
                  header: 'Баланс после',
                  align: 'right',
                  render: (l) => (
                    <span className="tabular-nums">
                      {formatRubles(l.balance_after_kopecks ?? 0)}
                    </span>
                  ),
                },
                { key: 'comment', header: 'Комментарий', render: (l) => l.comment ?? '—' },
                {
                  key: 'created_at',
                  header: 'Дата',
                  render: (l) => (l.created_at ? formatDateTime(l.created_at) : '—'),
                },
              ]}
              rows={ledgerItems}
              rowKey={(l) => l.id ?? `${l.type}-${l.created_at}`}
              emptyTitle="Операций нет"
              emptyHint="Измените фильтры"
            />
          )}

          <Pagination
            total={ledger.data?.total ?? 0}
            offset={ledgerOffset}
            limit={PAGE_SIZE}
            onChange={setLedgerOffset}
          />
        </Card>
      )}

      {tab === 'earnings' && (
        <Card title="Начисления" subtitle={`Всего начислений: ${earnings.data?.total ?? 0}`}>
          <div className={TOOLBAR_CLASSES}>
            <Field label="Партнёр" className={TOOLBAR_FIELD_CLASSES}>
              <Select value={earnPartner} onChange={(event) => setEarnPartner(event.target.value)}>
                <option value="">Все партнёры</option>
                {partners.data?.items?.map((partner) => (
                  <option key={partner.id} value={partner.id}>
                    {partner.name ?? partner.email ?? partner.id}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Оффер" className={TOOLBAR_FIELD_CLASSES}>
              <Select value={earnOffer} onChange={(event) => setEarnOffer(event.target.value)}>
                <option value="">Все офферы</option>
                {offers.data?.items?.map((offer) => (
                  <option key={offer.id} value={offer.id}>
                    {offer.project_name ? `${offer.project_name} — ` : ''}
                    {offer.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="С даты" className={TOOLBAR_FIELD_CLASSES}>
              <Input
                type="date"
                value={earnFrom}
                onChange={(event) => setEarnFrom(event.target.value)}
              />
            </Field>
            <Field label="По дату" className={TOOLBAR_FIELD_CLASSES}>
              <Input
                type="date"
                value={earnTo}
                onChange={(event) => setEarnTo(event.target.value)}
              />
            </Field>
          </div>

          {earnings.isLoading && earningsItems.length === 0 ? (
            <div className={SKELETON_CLASSES}>
              <Skeleton style={{ height: 36 }} />
              <Skeleton style={{ height: 36 }} />
              <Skeleton style={{ height: 36 }} />
            </div>
          ) : (
            <Table
              columns={[
                {
                  key: 'partner',
                  header: 'Партнёр',
                  render: (e) => partnerName(e.partner_id) ?? e.partner_id ?? '—',
                },
                {
                  key: 'offer',
                  header: 'Оффер',
                  render: (e) => offerName(e.offer_id) ?? e.offer_id ?? '—',
                },
                {
                  key: 'amount',
                  header: 'Сумма',
                  align: 'right',
                  render: (e) => (
                    <span className="tabular-nums">{formatRubles(e.amount_kopecks ?? 0)}</span>
                  ),
                },
                {
                  key: 'rate',
                  header: 'Ставка',
                  align: 'right',
                  render: (e) => formatBps(e.rate_bps ?? 0),
                },
                {
                  key: 'external_user_id',
                  header: 'Внешний пользователь',
                  render: (e) => (
                    <span className="tabular-nums">{e.external_user_id ?? '—'}</span>
                  ),
                },
                {
                  key: 'reversed',
                  header: 'Статус',
                  render: (e) =>
                    e.reversed ? (
                      <Badge tone="danger">Отменено</Badge>
                    ) : (
                      <Badge tone="success">Активно</Badge>
                    ),
                },
                {
                  key: 'created_at',
                  header: 'Дата',
                  render: (e) => (e.created_at ? formatDateTime(e.created_at) : '—'),
                },
              ]}
              rows={earningsItems}
              rowKey={(e) => e.id ?? `${e.partner_id}-${e.created_at}`}
              emptyTitle="Начислений нет"
              emptyHint="Измените фильтры"
            />
          )}

          <Pagination
            total={earnings.data?.total ?? 0}
            offset={earnOffset}
            limit={PAGE_SIZE}
            onChange={setEarnOffset}
          />
        </Card>
      )}
    </div>
  )
}