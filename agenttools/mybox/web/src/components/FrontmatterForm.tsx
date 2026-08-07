import { ReactNode } from 'react'

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
  'project',
  'type',
  'created',
  'lastmod',
  'id',
]

const KNOWN = new Set(FIELD_ORDER)

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
      <input type="checkbox" checked={value} onChange={(e) => onChange(e.target.checked)} aria-label={ariaLabel} />
    ) : (
      <input
        type="text"
        value={typeof value === 'string' ? value : genericDisplay(value)}
        onChange={(e) => onChange(e.target.value)}
        aria-label={ariaLabel}
      />
    ))
  return (
    <label className="field">
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
    <label className="field">
      <span>{label}</span>
      <select value={str(value)} onChange={(e) => onChange(e.target.value)} aria-label={ariaLabel}>
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

function ListField({ label, value, onChange, ariaLabel }: FieldProps) {
  const items = Array.isArray(value) ? value.map(String) : []
  return (
    <label className="field">
      <span>{label}</span>
      <input
        type="text"
        value={joinList(items)}
        onChange={(e) => onChange(splitList(e.target.value))}
        aria-label={ariaLabel}
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
    <div className="frontmatter-form">
      <div className="field-row">
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
      <div className="field-row">
        <ListField label="Tags" value={value.tags} onChange={(v) => setField('tags', v)} ariaLabel="Metadata tags" />
        <ListField
          label="Aliases"
          value={value.aliases}
          onChange={(v) => setField('aliases', v)}
          ariaLabel="Metadata aliases"
        />
      </div>
      <div className="field-row">
        <Field label="Assignee" value={value.assignee} onChange={(v) => setField('assignee', v)} ariaLabel="Metadata assignee" />
        <Field label="Due" value={value.due} onChange={(v) => setField('due', v)} ariaLabel="Metadata due" />
      </div>
      <div className="field-row">
        <Field label="Project" value={value.project} onChange={(v) => setField('project', v)} ariaLabel="Metadata project" />
        <Field label="Type" value={value.type} onChange={(v) => setField('type', v)} ariaLabel="Metadata type" />
      </div>
      <div className="field-row">
        <Field label="Created" value={value.created} onChange={(v) => setField('created', v)} ariaLabel="Metadata created" />
        <Field label="Last modified" value={value.lastmod} onChange={(v) => setField('lastmod', v)} ariaLabel="Metadata lastmod" />
      </div>
      <Field label="ID" value={value.id} onChange={(v) => setField('id', v)} ariaLabel="Metadata id" />

      {extraKeys.length > 0 && (
        <div className="extra-fields">
          <h4>Other metadata</h4>
          {extraKeys.map((k) => (
            <div className="extra-field-row" key={k}>
              <input
                className="extra-key"
                defaultValue={k}
                aria-label="Metadata key"
                onBlur={(e) => renameField(k, e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') e.currentTarget.blur()
                }}
              />
              <input
                className="extra-value"
                value={genericDisplay(value[k])}
                aria-label="Metadata value"
                onChange={(e) => setField(k, genericParse(value[k], e.target.value))}
              />
              <button type="button" className="ghost" onClick={() => removeField(k)} aria-label={`Remove metadata ${k}`}>
                ×
              </button>
            </div>
          ))}
        </div>
      )}
      <button type="button" className="ghost" onClick={addField}>
        + Add metadata
      </button>
    </div>
  )
}

function order(key: string): number {
  const i = FIELD_ORDER.indexOf(key)
  return i === -1 ? 1000 : i
}

export function FrontmatterSummary({ data }: { data: Record<string, unknown> }) {
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
      return <span className={`badge status-${v}`}>{v}</span>
    }
    if (k === 'priority' && typeof v === 'string') {
      return <span className={`badge priority-${v}`}>{v}</span>
    }
    if (Array.isArray(v)) {
      return v.map((x, i) => (
        <span key={i} className="badge tag">
          {String(x)}
        </span>
      ))
    }
    return String(v)
  }

  return (
    <div className="frontmatter-card">
      <h3>Metadata</h3>
      <dl className="frontmatter-grid">
        {entries.map(([k, v]) => (
          <div className="frontmatter-row" key={k}>
            <dt className="frontmatter-key">{k}</dt>
            <dd className="frontmatter-value">{renderValue(k, v)}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}
