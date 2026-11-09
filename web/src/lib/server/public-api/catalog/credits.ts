import { z } from 'zod';

import { CapabilityAnsweredBy, CapabilityEstimatedLatency, CapabilitySideEffect } from './protocol';
import type { CapabilityToolDefinition } from './definition';
import { companyCreditsResultSchema } from '$lib/company/credits-contract';

export const companyCreditsGetInputSchema = z.strictObject({});

export const creditsToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'company_credits_get',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_credits_get',
    description: "How the company pays for AI and what it has left: whether a subscription, the company's own key or nothing pays, the subscription credits left this period, when it ends and how many credits each billed seat brings, the purchased credits left, the most one member may spend in a period, and what the requester spent. An administrator also gets the billed seats and every member's spending; anybody else gets an empty members list and no seat count. Read this to answer 'how many credits are left' or 'how much have I used'.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyCreditsGetInputSchema,
    result: { schema: companyCreditsResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
];
