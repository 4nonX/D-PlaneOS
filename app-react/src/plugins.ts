import type { NavItem } from './components/layout/navConfig'

// Open-source version has no injected enterprise plugins

/**
 * Plugin extension point: inject additional nav items.
 * External plugins call this to add sidebar entries.
 * @param nav - the mutable nav array to push items into
 */
export function pluginNavInject(_nav: NavItem[]) {}

/**
 * Plugin extension point: inject additional Settings panel sections.
 * Return a React node to render inside the Settings page.
 */
export function pluginSettingsInject(_setters: any) { return null }

/**
 * Plugin extension point: inject additional protected routes.
 * Return an array of TanStack Router route objects.
 */
export function getPluginRoutes(_protectedRoute: any): any[] {
  return []
}
