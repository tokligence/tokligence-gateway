/**
 * Edition types for Tokligence Gateway
 */
export type Edition = 'personal' | 'team' | 'enterprise'

/**
 * Feature flags based on edition
 */
export interface EditionFeatures {
  // Marketplace access
  marketplaceConsumer: boolean // Can browse and buy tokens
  marketplaceProvider: boolean // Can publish and sell tokens

  // User management
  multiUser: boolean
  userRoles: boolean
  adminPanel: boolean

  // Authentication
  authRequired: boolean
  sso: boolean

  // Advanced features
  teams: boolean
  projects: boolean
  auditLogs: boolean
  advancedAnalytics: boolean
  customBranding: boolean

  // API features
  apiKeyManagement: 'self' | 'admin' | 'scoped'
}

/**
 * Edition detection response from backend
 */
export interface EditionInfo {
  edition: Edition
  features: EditionFeatures
  marketplaceEnabled: boolean
}

/**
 * Get default features for an edition
 */
export function getEditionFeatures(edition: Edition): EditionFeatures {
  switch (edition) {
    case 'personal':
      return {
        marketplaceConsumer: true, // CAN browse and buy
        marketplaceProvider: false, // CANNOT publish and sell
        multiUser: false,
        userRoles: false,
        adminPanel: false,
        authRequired: false,
        sso: false,
        teams: false,
        projects: false,
        auditLogs: false,
        advancedAnalytics: false,
        customBranding: false,
        apiKeyManagement: 'self',
      }
    case 'team':
      return {
        marketplaceConsumer: true,
        marketplaceProvider: true, // Full marketplace access
        multiUser: true,
        userRoles: true,
        adminPanel: true,
        authRequired: true,
        sso: false,
        teams: false,
        projects: false,
        auditLogs: true,
        advancedAnalytics: false,
        customBranding: false,
        apiKeyManagement: 'admin',
      }
    case 'enterprise':
      return {
        marketplaceConsumer: true,
        marketplaceProvider: true,
        multiUser: true,
        userRoles: true,
        adminPanel: true,
        authRequired: true,
        sso: true,
        teams: true,
        projects: true,
        auditLogs: true,
        advancedAnalytics: true,
        customBranding: true,
        apiKeyManagement: 'scoped',
      }
  }
}

/**
 * Detect edition from environment or config
 * This is a client-side heuristic until we have a proper backend API
 */
export function detectEdition(profile: { user: { roles: string[] } } | null): Edition {
  // For now, use auth status and roles as heuristic
  if (!profile) {
    return 'personal' // No auth = personal edition
  }

  const roles = profile.user.roles

  // Check for enterprise indicators (SSO, teams, etc.)
  // This will be improved when backend provides edition info
  if (roles.includes('enterprise_admin')) {
    return 'enterprise'
  }

  // Check for team edition (auth required, multi-user)
  if (roles.includes('root_admin') || roles.includes('gateway_admin')) {
    return 'team'
  }

  // Default to team if authenticated (we enable auth = team edition)
  return 'team'
}
