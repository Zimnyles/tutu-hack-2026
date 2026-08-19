const API_BASE = '/api/v1'

export class APIError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
  ) {
    super(message)
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) {
    headers.set('Content-Type', 'application/json')
  }
  if ((init.method ?? 'GET') !== 'GET') {
    headers.set('X-CSRF-Token', readCookie('ow_csrf'))
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  })

  if (!response.ok) {
    const body = await response.json().catch(() => null)
    const code = body?.error?.code ?? 'INTERNAL_ERROR'
    const message = body?.error?.message ?? 'Не удалось выполнить запрос'
    throw new APIError(code, message, response.status)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

function readCookie(name: string): string {
  const prefix = `${name}=`
  const item = document.cookie.split('; ').find((value) => value.startsWith(prefix))
  return item?.slice(prefix.length) ?? ''
}
