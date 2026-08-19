import { useQuery } from '@tanstack/react-query'

type WikipediaPage = {
  thumbnail?: { source: string }
}

type WikipediaResponse = {
  query?: { pages?: Record<string, WikipediaPage> }
}

const endpoint = 'https://ru.wikipedia.org/w/api.php'
const day = 86_400_000

async function requestThumbnail(parameters: Record<string, string>, size: number) {
  const query = new URLSearchParams({
    action: 'query',
    format: 'json',
    origin: '*',
    prop: 'pageimages',
    piprop: 'thumbnail',
    pithumbsize: String(size),
    ...parameters,
  })

  const response = await fetch(`${endpoint}?${query}`)
  if (!response.ok) {
    throw new Error('Не удалось получить фотографию города')
  }

  const payload: WikipediaResponse = await response.json()

  return Object.values(payload.query?.pages ?? {}).find((page) => page.thumbnail)?.thumbnail?.source ?? null
}

// Названия городов в каталоге не всегда совпадают с заголовками статей,
// поэтому при промахе по точному заголовку идём через поиск.
async function findCityPhoto(name: string, size: number) {
  const exact = await requestThumbnail({ titles: name, redirects: '1' }, size)
  if (exact) {
    return exact
  }

  return requestThumbnail({ generator: 'search', gsrsearch: `${name} город`, gsrlimit: '1' }, size)
}

export function useCityPhoto(name: string, size = 900) {
  return useQuery({
    queryKey: ['city-photo', name, size],
    queryFn: () => findCityPhoto(name, size),
    enabled: Boolean(name),
    staleTime: day,
    gcTime: day,
    retry: false,
  })
}
