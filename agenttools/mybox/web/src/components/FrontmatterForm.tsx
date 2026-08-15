import { ReactNode } from 'react'
import { Input } from './ui/input'
import { Button } from './ui/button'
import { PriorityBadge, StatusBadge, TagBadge } from './badges'

export const STATUS_OPTIONS = ['todo', 'doing', 'blocked', 'review', 'done']
export const PRIORITY_OPTIONS = ['low', 'medium', 'high', 'urgent']

const FIELD_ORDER = [
  'title',
  'status',
  'priority',
  'tags',
  'aliases',
  'assignee',
  'due',
  'type',
  'pending_until',
  'pending_reason',
]

const KNOWN = new Set(FIELD_ORDER)

const fieldClasses =
  'field flex min-w-[140px] flex-1 flex-col gap-1 text-[13px] font-semibold'

const controlClasses =
  'h-9 rounded-md border border-input bg-card px-3 text-sm font-normal text-foreground transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50'

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function splitList(text: string): string[] {
  return text.split(',').map((s) => s.trim())
}

function joinList(values: string[]): string {
  return values.join(', ')
}

function genericDisplay(v: unknown): string {
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  if (Array.isArray(v)) return joinList(v.map(String))
  return JSON.stringify(v)
}

function genericParse(original: unknown, text: string): unknown {
  if (Array.isArray(original)) {
    return splitList(text)
  }
  if (text.trim() === '') return ''
  if (typeof original === 'object' && original !== null) {
    try {
      return JSON.parse(text)
    } catch {
      return text
    }
  }
  if (typeof original === 'number' || typeof original === 'boolean') {
    try {
      return JSON.parse(text)
    } catch {
      return text
    }
  }
  return text
}

interface FieldProps {
  label: string
  value: unknown
  onChange: (v: unknown) => void
  ariaLabel: string
  children?: ReactNode
}

function Field({ label, value, onChange, ariaLabel, children }: FieldProps) {
  const input =
    children ??
    (typeof value === 'boolean' ? (
      <input
        type="checkbox"
        className="h-4 w-4 rounded border border-input accent-primary"
        checked={value}
        onChange={(e) => onChange(e.target.checked)}
        aria-label={ariaLabel}
      />
    ) : (
      <Input
        type="text"
        value={typeof value === 'string' ? value : genericDisplay(value)}
        onChange={(e) => onChange(e.target.value)}
        aria-label={ariaLabel}
        className="font-normal"
      />
    ))
  return (
    <label className={fieldClasses}>
      <span>{label}</span>
      {input}
    </label>
  )
}

interface SelectFieldProps extends Omit<FieldProps, 'children'> {
  options: string[]
}

function SelectField({ label, value, onChange, ariaLabel, options }: SelectFieldProps) {
  return (
    <label className={fieldClasses}>
      <span>{label}</span>
      <select value={str(value)} onChange={(e) => onChange(e.target.value)} aria-label={ariaLabel} className={controlClasses}>
        <option value="">—</option>
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
    </label>
  )
}

function toISODate(v: unknown): string {
  if (typeof v !== 'string' || !v.trim()) return ''
  const s = v.trim()
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  if (/^\d{8}$/.test(s)) {
    return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
  }
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return ''
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function DateField({ label, value, onChange, ariaLabel, compact }: FieldProps & { compact?: boolean }) {
  return (
    <label className={fieldClasses}>
      <span>{label}</span>
      <input
        type="date"
        value={toISODate(value)}
        onChange={(e) => onChange(compact ? e.target.value.replace(/-/g, '') : e.target.value)}
        aria-label={ariaLabel}
        className={controlClasses}
      />
    </label>
  )
}

function ListField({ label, value, onChange, ariaLabel }: FieldProps) {
  const items = Array.isArray(value) ? value.map(String) : []
  return (
    <label className={fieldClasses}>
      <span>{label}</span>
      <Input
        type="text"
        value={joinList(items)}
        onChange={(e) => onChange(splitList(e.target.value))}
        aria-label={ariaLabel}
        className="font-normal"
      />
    </label>
  )
}

interface FrontmatterFormProps {
  value: Record<string, unknown>
  onChange: (next: Record<string, unknown>) => void
}

export function FrontmatterForm({ value, onChange }: FrontmatterFormProps) {
  const setField = (key: string, v: unknown) => onChange({ ...value, [key]: v })
  const removeField = (key: string) => {
    const next = { ...value }
    delete next[key]
    onChange(next)
  }
  const renameField = (oldKey: string, newKey: string) => {
    const trimmed = newKey.trim()
    if (trimmed === '' || trimmed === oldKey) return
    const next = { ...value }
    next[trimmed] = next[oldKey]
    delete next[oldKey]
    onChange(next)
  }
  const addField = () => {
    const base = 'field'
    let key = base
    let n = 1
    while (key in value) {
      key = `${base}_${n++}`
    }
    onChange({ ...value, [key]: '' })
  }
  const extraKeys = Object.keys(value).filter((k) => !KNOWN.has(k))

  return (
    <div className="frontmatter-form flex flex-col gap-3">
      <div className="field-row flex flex-wrap gap-3">
        <Field label="Title" value={value.title} onChange={(v) => setField('title', v)} ariaLabel="Metadata title" />
        <SelectField
          label="Status"
          value={value.status}
          onChange={(v) => setField('status', v)}
          ariaLabel="Metadata status"
          options={STATUS_OPTIONS}
        />
        <SelectField
          label="Priority"
          value={value.priority}
          onChange={(v) => setField('priority', v)}
          ariaLabel="Metadata priority"
          options={PRIORITY_OPTIONS}
        />
      </div>
      <div className="field-row flex flex-wrap gap-3">
        <ListField label="Tags" value={value.tags} onChange={(v) => setField('tags', v)} ariaLabel="Metadata tags" />
        <ListField
          label="Aliases"
          value={value.aliases}
          onChange={(v) => setField('aliases', v)}
          ariaLabel="Metadata aliases"
        />
      </div>
      <div className="field-row flex flex-wrap gap-3">
        <Field label="Assignee" value={value.assignee} onChange={(v) => setField('assignee', v)} ariaLabel="Metadata assignee" />
        <DateField label="Due" value={value.due} onChange={(v) => setField('due', v)} ariaLabel="Metadata due" />
      </div>
      <div className="field-row flex flex-wrap gap-3">
        <Field label="Type" value={value.type} onChange={(v) => setField('type', v)} ariaLabel="Metadata type" />
      </div>
      <div className="field-row flex flex-wrap gap-3">
        <DateField
          compact
          label="Pending until"
          value={value.pending_until}
          onChange={(v) => setField('pending_until', v)}
          ariaLabel="Metadata pending until"
        />
        <Field
          label="Pending reason"
          value={value.pending_reason}
          onChange={(v) => setField('pending_reason', v)}
          ariaLabel="Metadata pending reason"
        />
      </div>

      {extraKeys.length > 0 && (
        <div className="extra-fields mb-2.5 mt-1">
          <h4 className="mb-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase">
            Other metadata
          </h4>
          {extraKeys.map((k) => (
            <div className="extra-field-row mb-1.5 flex gap-2" key={k}>
              <Input
                className="extra-key grow-0 basis-[160px] font-mono text-[13px]"
                defaultValue={k}
                aria-label="Metadata key"
                onBlur={(e) => renameField(k, e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') e.currentTarget.blur()
                }}
              />
              <Input
                className="extra-value flex-1 font-mono text-[13px]"
                value={genericDisplay(value[k])}
                aria-label="Metadata value"
                onChange={(e) => setField(k, genericParse(value[k], e.target.value))}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                onClick={() => removeField(k)}
                aria-label={`Remove metadata ${k}`}
              >
                ×
              </Button>
            </div>
          ))}
        </div>
      )}
      <Button type="button" variant="ghost" size="sm" className="justify-start" onClick={addField}>
        + Add metadata
      </Button>
    </div>
  )
}

function order(key: string): number {
  const i = FIELD_ORDER.indexOf(key)
  return i === -1 ? 1000 : i
}

export function FrontmatterSummary({ data }: { data: Record<string, unknown> }) {
  const URL_RE = /^(https?:\/\/|ftp:\/\/|mailto:)/i

  const entries = Object.entries(data)
    .filter(([, v]) => {
      if (v === undefined || v === null) return false
      if (typeof v === 'string' && v.trim() === '') return false
      if (Array.isArray(v) && v.length === 0) return false
      return true
    })
    .sort((a, b) => order(a[0]) - order(b[0]))
  if (entries.length === 0) return null

  const renderValue = (k: string, v: unknown): ReactNode => {
    if (k === 'status' && typeof v === 'string') {
      return <StatusBadge status={v} />
    }
    if (k === 'priority' && typeof v === 'string') {
      return <PriorityBadge priority={v} />
    }
    if (Array.isArray(v)) {
      return v.map((x, i) => (
        <TagBadge key={i}>{String(x)}</TagBadge>
      ))
    }
    if (typeof v === 'string' && URL_RE.test(v.trim())) {
      const href = v.trim()
      return (
        <a href={href} target="_blank" rel="noopener noreferrer" className="text-primary underline underline-offset-4">
          {href}
        </a>
      )
    }
    return String(v)
  }

  return (
    <div className="frontmatter-card mb-4 rounded-lg border bg-muted/40 p-3">
      <h3 className="mb-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase">Metadata</h3>
      <dl className="frontmatter-grid m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
        {entries.map(([k, v]) => (
          <div className="frontmatter-row contents" key={k}>
            <dt className="frontmatter-key text-xs font-semibold whitespace-nowrap text-muted-foreground">{k}</dt>
            <dd className="frontmatter-value m-0 flex flex-wrap gap-1.5 text-[13px]">{renderValue(k, v)}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}
