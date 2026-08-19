import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ArrowRight, Eye, EyeOff, LoaderCircle, LockKeyhole, Mail } from 'lucide-react'
import { api } from '../../api'
import { Logo } from '../../components/Logo'
import type { User } from '../../types'

type Mode = 'login' | 'register'

export function AuthScreen({ onAuthenticated }: { onAuthenticated: () => void }) {
  const [mode, setMode] = useState<Mode>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [showPassword, setShowPassword] = useState(false)

  const mutation = useMutation({
    mutationFn: () => {
      const path = mode === 'login' ? '/auth/login' : '/auth/register'
      const body = mode === 'login'
        ? { email, password }
        : { email, password, display_name: displayName }

      return api<{ user: User }>(path, {
        method: 'POST',
        body: JSON.stringify(body),
      })
    },
    onSuccess: onAuthenticated,
  })

  return (
    <main className="auth-page">
      <section className="auth-art">
        <Logo inverse />
        <div className="auth-orbit">
          <div className="mini-world">
            <span className="land land-one" />
            <span className="land land-two" />
            <span className="pin-dot" />
          </div>
          <strong>Открывайте Россию<br />город за городом</strong>
        </div>
        <div className="auth-color-line"><span /><span /><span /><span /></div>
      </section>

      <section className="auth-panel">
        <div className="auth-copy">
          <span className="eyebrow">Открывай</span>
          <h1>{mode === 'login' ? 'С возвращением' : 'Создать аккаунт'}</h1>
          <p>{mode === 'login' ? 'Продолжайте с того места, где остановились' : 'Ваши поездки и открытия будут в одном месте'}</p>
        </div>

        <div className="segmented-control" role="tablist" aria-label="Вход или регистрация">
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'login'}
            className={mode === 'login' ? 'active' : ''}
            onClick={() => setMode('login')}
          >
            Войти
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'register'}
            className={mode === 'register' ? 'active' : ''}
            onClick={() => setMode('register')}
          >
            Регистрация
          </button>
        </div>

        <form
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          {mode === 'register' && (
            <label>
              Имя
              <input
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                autoComplete="name"
                required
              />
            </label>
          )}
          <label>
            Email
            <div className="field-with-icon">
              <Mail aria-hidden="true" />
              <input
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                type="email"
                autoComplete="email"
                inputMode="email"
                maxLength={254}
                placeholder="name@example.ru"
                required
              />
            </div>
          </label>
          <label>
            Пароль
            <div className="field-with-icon">
              <LockKeyhole aria-hidden="true" />
              <input
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                type={showPassword ? 'text' : 'password'}
                minLength={8}
                maxLength={128}
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                placeholder="Минимум 8 символов"
                required
              />
              <button
                type="button"
                onClick={() => setShowPassword((value) => !value)}
                aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
              >
                {showPassword ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
              </button>
            </div>
          </label>

          {mutation.error && <div className="form-error" role="alert">{mutation.error.message}</div>}

          <button className="primary-button full-width" disabled={mutation.isPending}>
            {mutation.isPending ? <LoaderCircle className="spin" aria-hidden="true" /> : <ArrowRight aria-hidden="true" />}
            {mode === 'login' ? 'Войти' : 'Создать аккаунт'}
          </button>
          {mode === 'register' && (
            <p className="field-hint">Создавая аккаунт, вы соглашаетесь на обработку данных для подбора поездок.</p>
          )}
        </form>

      </section>
    </main>
  )
}
