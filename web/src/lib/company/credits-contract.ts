import { z } from 'zod';

export const companyModelPayers = ['subscription', 'company_key', 'nothing'] as const;

export const shownModelPayers = [...companyModelPayers, 'unknown'] as const;

export type CompanyModelPayer = (typeof companyModelPayers)[number];

export type ShownModelPayer = (typeof shownModelPayers)[number];

export type WhatPaysForModels = { holdsKey: boolean | null; usesOwnKey: boolean; creditsCanPay: boolean };

export function payerOfTheModels({ holdsKey, usesOwnKey, creditsCanPay }: WhatPaysForModels): ShownModelPayer {
  if (usesOwnKey && holdsKey === true) return 'company_key';
  if (usesOwnKey && holdsKey === null) return 'unknown';
  return payerOfCredits(creditsCanPay);
}

export function payerTheShippedContractCanSay(facts: WhatPaysForModels): CompanyModelPayer {
  const shown = payerOfTheModels(facts);
  return shown === 'unknown' ? payerOfCredits(facts.creditsCanPay) : shown;
}

function payerOfCredits(creditsCanPay: boolean): CompanyModelPayer {
  return creditsCanPay ? 'subscription' : 'nothing';
}

export function isAModelKeyHeld(isHeldByThePlane: boolean, isHeldByTheHost: boolean | null): boolean | null {
  return isHeldByThePlane || isHeldByTheHost;
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

export function holdsAnOpenSubscription(subscriptionStatus: string | null): boolean {
  return subscriptionStatus !== null && !endedSubscriptionStatuses.has(subscriptionStatus);
}
