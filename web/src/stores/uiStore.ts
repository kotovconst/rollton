import { create } from 'zustand'

interface UiState {
  // Stub: grows as real UI features land.
  activeModal: string | null
  setActiveModal: (m: string | null) => void
}

export const useUiStore = create<UiState>((set) => ({
  activeModal: null,
  setActiveModal: (activeModal) => set({ activeModal }),
}))
