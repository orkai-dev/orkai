/**
 * Central registry of API endpoint paths (F-05).
 *
 * Endpoints are expressed as builder functions instead of string literals
 * scattered across hooks, so the API version, base path, or a provider-namespaced
 * route can change in one place, typos are caught by the compiler, and there is a
 * single source of truth for the API surface. This also makes an eventual
 * OpenAPI-typed client a drop-in replacement.
 *
 * Convention: one nested object per domain; each value is a function returning
 * the path. Query strings are appended by callers.
 */

export const API_VERSION = "v1";

/** Base path for all versioned API routes. */
export const API_BASE = `/api/${API_VERSION}`;

const b = API_BASE;

export const endpoints = {
  auth: {
    setupStatus: () => `${b}/auth/setup-status`,
    register: () => `${b}/auth/register`,
    login: () => `${b}/auth/login`,
    refresh: () => `${b}/auth/refresh`,
    me: () => `${b}/auth/me`,
    profile: () => `${b}/auth/profile`,
    changePassword: () => `${b}/auth/change-password`,
    avatars: () => `${b}/auth/avatars`,
    oauth2fa: () => `${b}/auth/oauth/2fa`,
    providers: () => `${b}/auth/providers`,
    twoFASetup: () => `${b}/auth/2fa/setup`,
    twoFAVerify: () => `${b}/auth/2fa/verify`,
    twoFADisable: () => `${b}/auth/2fa/disable`,
  },

  apps: {
    list: () => `${b}/apps`,
    create: () => `${b}/apps`,
    detail: (id: string) => `${b}/apps/${id}`,
    capabilities: (id: string) => `${b}/apps/${id}/capabilities`,
    status: (id: string) => `${b}/apps/${id}/status`,
    pods: (id: string) => `${b}/apps/${id}/pods`,
    podEvents: (id: string, podName: string) => `${b}/apps/${id}/pods/${podName}/events`,
    deployments: (id: string) => `${b}/apps/${id}/deployments`,
    deploy: (id: string) => `${b}/apps/${id}/deploy`,
    clearCache: (id: string) => `${b}/apps/${id}/clear-cache`,
    restart: (id: string) => `${b}/apps/${id}/restart`,
    stop: (id: string) => `${b}/apps/${id}/stop`,
    scale: (id: string) => `${b}/apps/${id}/scale`,
    env: (id: string) => `${b}/apps/${id}/env`,
    secrets: (id: string) => `${b}/apps/${id}/secrets`,
    domains: (id: string) => `${b}/apps/${id}/domains`,
    generateDomain: (id: string) => `${b}/apps/${id}/domains/generate`,
    webhook: (id: string) => `${b}/apps/${id}/webhook`,
    webhookEnable: (id: string) => `${b}/apps/${id}/webhook/enable`,
    webhookDisable: (id: string) => `${b}/apps/${id}/webhook/disable`,
    webhookRegenerate: (id: string) => `${b}/apps/${id}/webhook/regenerate`,
  },

  projects: {
    apps: (projectId: string) => `${b}/projects/${projectId}/apps`,
  },

  pages: {
    list: () => `${b}/pages`,
  },

  workers: {
    list: () => `${b}/workers`,
  },

  databases: {
    list: () => `${b}/databases`,
  },

  deployments: {
    detail: (id: string) => `${b}/deployments/${id}`,
    cancel: (id: string) => `${b}/deployments/${id}/cancel`,
  },

  domains: {
    detail: (id: string) => `${b}/domains/${id}`,
  },

  notifications: {
    channels: () => `${b}/notifications/channels`,
    events: () => `${b}/notifications/events`,
    test: () => `${b}/notifications/test`,
  },

  settings: {
    smtp: () => `${b}/settings/smtp`,
    smtpTest: () => `${b}/settings/smtp/test`,
  },

  system: {
    upgrade: () => `${b}/system/upgrade`,
    backupConfig: () => `${b}/system/backup/config`,
    backupList: () => `${b}/system/backup/list`,
    backupTrigger: () => `${b}/system/backup/trigger`,
    restoreScan: () => `${b}/system/restore/scan`,
    restoreExecute: () => `${b}/system/restore/execute`,
  },

  version: {
    get: () => `${b}/version`,
  },

  apiKeys: {
    list: () => `${b}/api-keys`,
    create: () => `${b}/api-keys`,
    revoke: (id: string) => `${b}/api-keys/${id}`,
  },
} as const;
