import { createContext, useContext } from 'react'
import type { ProfileResponse } from '../types/api'

const ProfileContext = createContext<ProfileResponse | null>(null)

export function ProfileProvider({ value, children }: { value: ProfileResponse | null; children: React.ReactNode }) {
  return <ProfileContext.Provider value={value}>{children}</ProfileContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useProfileContext(): ProfileResponse | null {
  return useContext(ProfileContext)
}

export default ProfileContext
