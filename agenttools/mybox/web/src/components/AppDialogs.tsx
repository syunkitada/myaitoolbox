import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { useEscapeKey } from '../hooks/use-escape-key'

type DialogChoice = 'primary' | 'secondary' | 'cancel'

type DialogState =
  | { kind: 'prompt'; message: string; defaultValue: string }
  | { kind: 'confirm'; message: string }
  | { kind: 'alert'; message: string }
  | { kind: 'progress'; message: string }
  | { kind: 'confirm3'; message: string; primary: string; secondary: string; cancel: string }

interface DialogsApi {
  prompt: (message: string, defaultValue?: string) => Promise<string | null>
  confirm: (message: string) => Promise<boolean>
  alert: (message: string) => Promise<void>
  showProgress: (message: string) => void
  hideProgress: () => void
  // confirm3 shows a dialog with three choices (e.g. Save / Discard / Cancel).
  confirm3: (
    message: string,
    options?: { primary?: string; secondary?: string; cancel?: string },
  ) => Promise<DialogChoice>
}

const DialogsContext = createContext<DialogsApi | null>(null)

export function DialogsProvider({ children }: { children: ReactNode }) {
  const [dialog, setDialog] = useState<DialogState | null>(null)
  const resolverRef = useRef<((value: string | null | boolean | void) => void) | null>(null)

  const close = useCallback((value: string | null | boolean | void) => {
    resolverRef.current?.(value)
    resolverRef.current = null
    setDialog(null)
  }, [])

  const prompt = useCallback((message: string, defaultValue = '') => {
    return new Promise<string | null>((resolve) => {
      resolverRef.current = (value) => resolve(value as string | null)
      setDialog({ kind: 'prompt', message, defaultValue })
    })
  }, [])

  const confirm = useCallback((message: string) => {
    return new Promise<boolean>((resolve) => {
      resolverRef.current = (value) => resolve(value === true)
      setDialog({ kind: 'confirm', message })
    })
  }, [])

  const alert = useCallback((message: string) => {
    return new Promise<void>((resolve) => {
      resolverRef.current = () => resolve()
      setDialog({ kind: 'alert', message })
    })
  }, [])

  const showProgress = useCallback((message: string) => {
    setDialog({ kind: 'progress', message })
  }, [])

  const hideProgress = useCallback(() => {
    setDialog((current) => (current?.kind === 'progress' ? null : current))
  }, [])

  const confirm3 = useCallback(
    (message: string, options: { primary?: string; secondary?: string; cancel?: string } = {}) => {
      return new Promise<DialogChoice>((resolve) => {
        resolverRef.current = (value) => resolve(value as DialogChoice)
        setDialog({
          kind: 'confirm3',
          message,
          primary: options.primary ?? 'Save',
          secondary: options.secondary ?? 'Discard',
          cancel: options.cancel ?? 'Cancel',
        })
      })
    },
    [],
  )

  const api = { prompt, confirm, alert, showProgress, hideProgress, confirm3 }
  const cancelValue =
    dialog?.kind === 'prompt' ? null : dialog?.kind === 'confirm' ? false : dialog?.kind === 'confirm3' ? 'cancel' : undefined

  useEscapeKey(() => close(cancelValue), Boolean(dialog) && dialog?.kind !== 'progress')

  return (
    <DialogsContext.Provider value={api}>
      {children}
      {dialog && (
        <div
          className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4"
          onClick={() => {
            if (dialog.kind !== 'progress') close(cancelValue)
          }}
        >
          <div
            className="w-full max-w-md rounded-lg border bg-card p-5 shadow-xl"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            data-testid="app-dialog"
            data-dialog-kind={dialog.kind}
            aria-label={dialog.kind === 'progress' ? 'Progress' : dialog.kind === 'prompt' ? 'Prompt' : dialog.kind === 'confirm' || dialog.kind === 'confirm3' ? 'Confirm' : 'Notice'}
          >
            <div className="mb-4 flex items-start gap-2">
              {dialog.kind === 'progress' ? (
                <Loader2 className="mt-0.5 size-5 shrink-0 animate-spin text-primary" aria-hidden="true" />
              ) : (
                <AlertTriangle className="mt-0.5 size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
              )}
              <div className="min-w-0">
                <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
                  {dialog.kind === 'progress' ? 'In progress' : dialog.kind === 'prompt' ? 'Prompt' : dialog.kind === 'confirm' || dialog.kind === 'confirm3' ? 'Confirm' : 'Notice'}
                </h2>
                <p className="mt-1 text-sm break-words whitespace-pre-wrap">{dialog.message}</p>
              </div>
            </div>
            {dialog.kind === 'progress' && (
              <p className="text-xs text-muted-foreground">Please keep this window open until the operation finishes.</p>
            )}
            {dialog.kind === 'prompt' && (
              <PromptField
                key={dialog.message + dialog.defaultValue}
                defaultValue={dialog.defaultValue}
                onConfirm={(v) => close(v)}
                onCancel={() => close(null)}
              />
            )}
            {dialog.kind === 'confirm' && (
              <div className="mt-5 flex justify-end gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  data-testid="app-dialog-cancel"
                  onClick={() => close(false)}
                >
                  Cancel
                </Button>
                <Button size="sm" data-testid="app-dialog-ok" autoFocus onClick={() => close(true)}>
                  OK
                </Button>
              </div>
            )}
            {dialog.kind === 'confirm3' && (
              <div className="mt-5 flex justify-end gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  data-testid="app-dialog-cancel"
                  onClick={() => close('cancel')}
                >
                  {dialog.cancel}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  data-testid="app-dialog-secondary"
                  onClick={() => close('secondary')}
                >
                  {dialog.secondary}
                </Button>
                <Button size="sm" data-testid="app-dialog-primary" autoFocus onClick={() => close('primary')}>
                  {dialog.primary}
                </Button>
              </div>
            )}
            {dialog.kind === 'alert' && (
              <div className="mt-5 flex justify-end gap-2">
                <Button size="sm" data-testid="app-dialog-ok" autoFocus onClick={() => close(undefined)}>
                  OK
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
    </DialogsContext.Provider>
  )
}

function PromptField({
  defaultValue,
  onConfirm,
  onCancel,
}: {
  defaultValue: string
  onConfirm: (value: string) => void
  onCancel: () => void
}) {
  const [value, setValue] = useState(defaultValue)

  return (
    <>
      <Input
        data-testid="app-dialog-input"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            onConfirm(value)
          }
        }}
        aria-label="Value"
        autoFocus
      />
      <div className="mt-5 flex justify-end gap-2">
        <Button variant="outline" size="sm" data-testid="app-dialog-cancel" onClick={onCancel}>
          Cancel
        </Button>
        <Button size="sm" data-testid="app-dialog-ok" onClick={() => onConfirm(value)}>
          OK
        </Button>
      </div>
    </>
  )
}

export function useDialogs(): DialogsApi {
  const ctx = useContext(DialogsContext)
  if (!ctx) throw new Error('useDialogs must be used within a DialogsProvider')
  return ctx
}
