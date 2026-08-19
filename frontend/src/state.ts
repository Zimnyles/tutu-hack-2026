import { create } from 'zustand'
import type { Bootstrap, PlanningTarget, Territory, Trip } from './types'

export type AppView = 'world' | 'trips' | 'community' | 'profile'

type AppState = {
  bootstrap: Bootstrap | null
  selected: Territory | null
  view: AppView
  planningTarget: PlanningTarget | null
  activeTrip: Trip | null
  toast: string
  searchOpen: boolean
  setBootstrap: (bootstrap: Bootstrap | null) => void
  setSelected: (territory: Territory | null) => void
  setView: (view: AppView) => void
  setPlanningTarget: (target: PlanningTarget | null) => void
  setActiveTrip: (trip: Trip | null) => void
  setToast: (message: string) => void
  setSearchOpen: (open: boolean) => void
}

export const useApp = create<AppState>((set) => ({
  bootstrap: null,
  selected: null,
  view: 'world',
  planningTarget: null,
  activeTrip: null,
  toast: '',
  searchOpen: false,
  setBootstrap: (bootstrap) => set({ bootstrap }),
  setSelected: (selected) => set({ selected }),
  setView: (view) => set({ view, selected: null }),
  setPlanningTarget: (planningTarget) => set({ planningTarget, searchOpen: Boolean(planningTarget) }),
  setActiveTrip: (activeTrip) => set({ activeTrip }),
  setToast: (toast) => set({ toast }),
  setSearchOpen: (searchOpen) => set({ searchOpen }),
}))
