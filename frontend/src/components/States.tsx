import { AlertCircle, LoaderCircle, RefreshCw } from 'lucide-react'

export function LoadingState({ label = 'Загружаем' }: { label?: string }) {
  return (
    <div className="inline-state" role="status" aria-live="polite">
      <LoaderCircle className="spin" aria-hidden="true" />
      <span>{label}</span>
    </div>
  )
}

export function ErrorState({ message, retry }: { message: string; retry?: () => void }) {
  return (
    <div className="empty-state error-state" role="alert">
      <AlertCircle aria-hidden="true" />
      <strong>Что-то пошло не так</strong>
      <p>{message}</p>
      {retry && (
        <button className="secondary-button" onClick={retry}>
          <RefreshCw aria-hidden="true" /> Повторить
        </button>
      )}
    </div>
  )
}

export function EmptyState({ title, text }: { title: string; text?: string }) {
  return (
    <div className="empty-state">
      <span className="empty-orbit" aria-hidden="true" />
      <strong>{title}</strong>
      {text && <p>{text}</p>}
    </div>
  )
}
