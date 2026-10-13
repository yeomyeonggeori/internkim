export const crmOrganizationStatuses = ['prospect', 'active', 'paused'] as const;
export const crmImportances = ['high', 'medium', 'low'] as const;

export type CRMOrganizationStatus = (typeof crmOrganizationStatuses)[number];
export type CRMImportance = (typeof crmImportances)[number];
