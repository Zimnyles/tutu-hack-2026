import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { AppNav } from '../../components/AppNav'
import { ErrorState, LoadingState } from '../../components/States'
import { useApp } from '../../state'
import type { Bootstrap, PublicSettings, User } from '../../types'
import { CommunityView } from '../community/CommunityView'
import { ProfileView } from '../profile/ProfileView'
import { SearchSheet } from '../recommendations/SearchSheet'
import { TripsView } from '../trips/TripsView'
import { WorldView } from './WorldView'

export function WorldApp({ user, settings }: { user: User; settings: PublicSettings }) {
  const view = useApp((state) => state.view)
  const setBootstrap = useApp((state) => state.setBootstrap)
  const bootstrap = useQuery({
    queryKey: ['world-bootstrap'],
    queryFn: () => api<Bootstrap>('/world/bootstrap'),
    refetchOnWindowFocus: false,
  })

  useEffect(() => { if (bootstrap.data) setBootstrap(bootstrap.data) }, [bootstrap.data, setBootstrap])

  return (
    <div className="app-shell">
      <AppNav user={user} />
      <main className="app-main">
        {bootstrap.isLoading && <LoadingState label="Открываем ваш мир" />}
        {bootstrap.isError && <ErrorState message={bootstrap.error.message} retry={() => bootstrap.refetch()} />}
        {bootstrap.data && view === 'world' && <WorldView data={bootstrap.data} />}
        {bootstrap.data && view === 'trips' && <TripsView />}
        {bootstrap.data && view === 'community' && <CommunityView season={bootstrap.data.season} />}
        {bootstrap.data && view === 'profile' && <ProfileView user={bootstrap.data.user} />}
      </main>
      {bootstrap.data && <SearchSheet user={bootstrap.data.user} settings={settings} />}
    </div>
  )
}
