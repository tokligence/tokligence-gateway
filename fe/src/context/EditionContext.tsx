/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useMemo, type PropsWithChildren } from 'react'
import { detectEdition, getEditionFeatures, type Edition, type EditionFeatures } from '../types/edition'
import { useProfileContext } from './ProfileContext'

interface EditionContextValue {
  edition: Edition
  features: EditionFeatures
}

const EditionContext = createContext<EditionContextValue | null>(null)

export function EditionProvider({ children }: PropsWithChildren) {
  const profile = useProfileContext()

  const value = useMemo<EditionContextValue>(() => {
    const edition = detectEdition(profile)
    const features = getEditionFeatures(edition)
    return { edition, features }
  }, [profile])

  return <EditionContext.Provider value={value}>{children}</EditionContext.Provider>
}

export function useEdition(): EditionContextValue {
  const context = useContext(EditionContext)
  if (!context) {
    throw new Error('useEdition must be used within EditionProvider')
  }
  return context
}

/**
 * Hook to check if a specific feature is enabled
 */
export function useFeature(feature: keyof EditionFeatures): boolean {
  const { features } = useEdition()
  return features[feature] as boolean
}
