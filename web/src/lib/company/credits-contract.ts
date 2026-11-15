import { z } from 'zod';

export const companyModelPayers = ['subscription', 'company_key', 'nothing'] as const;

export type CompanyModelPayer = (typeof companyModelPayers)[number];

export type WhatPaysForModels = { holdsKey: boolean; usesOwnKey: boolean; creditsCanPay: boolean };

export function payerOfTheModels({ holdsKey, usesOwnKey, creditsCanPay }: WhatPaysForModels): CompanyModelPayer {
  if (holdsKey && usesOwnKey) return 'company_key';
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

export function holdsAnOpenSubscription(subscriptionStatus: string | null): boolean {
  return subscriptionStatus !== null && !endedSubscriptionStatuses.has(subscriptionStatus);
}
