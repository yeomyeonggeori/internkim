import { z } from 'zod';

export const companyModelPayers = ['subscription', 'company_key', 'nothing'] as const;

export const shownModelPayers = [...companyModelPayers, 'unknown'] as const;

export type CompanyModelPayer = (typeof companyModelPayers)[number];

export type ShownModelPayer = (typeof shownModelPayers)[number];

export type WhatPaysForModels = { holdsKey: boolean | null; usesOwnKey: boolean; creditsCanPay: boolean };

export type ModelKeyReport = { holdsModelKey: boolean | null; usesOwnModelKeyAtStart: boolean | null } | null;

export function payerOfTheModels({ holdsKey, usesOwnKey, creditsCanPay }: WhatPaysForModels): ShownModelPayer {
  if (usesOwnKey && holdsKey === true) return 'company_key';
  if (usesOwnKey && holdsKey === null) return 'unknown';
  return payerOfCredits(creditsCanPay);
}

export function isAModelKeyHeld(isHeldByThePlane: boolean, isHeldByTheHost: boolean | null): boolean | null {
  return isHeldByThePlane || isHeldByTheHost;
}

export function payerRunning(report: ModelKeyReport, creditsCanPay: boolean): ShownModelPayer {
  if (report === null || report.holdsModelKey === null || report.usesOwnModelKeyAtStart === null) return 'unknown';
  return payerOfTheModels({ holdsKey: report.holdsModelKey, usesOwnKey: report.usesOwnModelKeyAtStart, creditsCanPay });
}

export function isPayerChangeWaiting(running: ShownModelPayer, intended: ShownModelPayer): boolean {
  if (running === 'unknown' || intended === 'unknown') return false;
  return (running === 'company_key') !== (intended === 'company_key');
}

export function payerTheShippedContractCanSay(shown: ShownModelPayer, creditsCanPay: boolean): CompanyModelPayer {
  return shown === 'unknown' ? payerOfCredits(creditsCanPay) : shown;
}

function payerOfCredits(creditsCanPay: boolean): CompanyModelPayer {
  return creditsCanPay ? 'subscription' : 'nothing';
}

export const companyCreditsResultSchema = z.strictObject({
  paidBy: z.enum(companyModelPayers),
  subscriptionStatus: z.string().nullable(),
  periodEndsAt: z.string().nullable(),
  billedSeats: z.number().int().nullable(),
  creditsPerSeat: z.number().int(),
  subscriptionCredits: z.number().int(),
  permanentCredits: z.number().int(),
  usageLimit: z.number().int(),
  myCreditsUsed: z.number().int(),
  members: z.array(z.strictObject({
    personID: z.string(),
    name: z.string(),
    creditsUsed: z.number().int(),
  })),
});

const endedSubscriptionStatuses = new Set(['canceled', 'incomplete_expired']);

const payingSubscriptionStatuses = new Set(['active', 'trialing']);

export function isSubscriptionPaying(subscriptionStatus: string | null): boolean {
  return subscriptionStatus !== null && payingSubscriptionStatuses.has(subscriptionStatus);
}

export function holdsAnOpenSubscription(subscriptionStatus: string | null): boolean {
  return subscriptionStatus !== null && !endedSubscriptionStatuses.has(subscriptionStatus);
}
