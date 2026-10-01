import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityAvailabilityState,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
  capabilityDescriptorSchema,
  capabilityIdempotencySchema,
  jsonValueSchema,
  resourceEffectContractSchema,
} from './protocol';
import { toolInvokeOutcomes } from './contract';
import { capabilityNamespaceSummaries, type CapabilityNamespace } from './namespaces';

export enum ResourceMutationEffect {
  Created = 'created',
  Sent = 'sent',
  Updated = 'updated',
  Previewed = 'previewed',
  Published = 'published',
  Deleted = 'deleted',
  Archived = 'archived',
}

export type CapabilityResultDefinition = {
  schema: z.ZodType;
  effects: Array<z.infer<typeof resourceEffectContractSchema>>;
  evidenceCondition?: {
    resultField: string;
    equals: z.infer<typeof jsonValueSchema>;
  };
};

type CapabilityToolCommonDefinition = {
  name: string;
  namespace: CapabilityNamespace;
  answeredBy: CapabilityAnsweredBy;
  privacyClass: string;
  policyResource: string;
  description: string;
  version: string;
  estimatedLatency: CapabilityEstimatedLatency;
  sideEffect: CapabilitySideEffect;
  idempotency?: z.infer<typeof capabilityIdempotencySchema>;
  requiresApproval?: boolean;
  requiresUserPresence?: boolean;
  requiresRequesterDevice?: boolean;
  approvalScope?: string;
  worksOffline?: boolean;
  completionEvidence?: {
    mode: string;
    action: string;
    targetKind: string;
  };
};

export type ContractedCapabilityToolDefinition = CapabilityToolCommonDefinition & {
  modelVisibility?: CapabilityModelVisibility;
  inputSchema: z.ZodType;
  inputIntentSchema?: z.ZodType;
  result: CapabilityResultDefinition;
};

export type UncontractedCapabilityToolDefinition = CapabilityToolCommonDefinition & {
  modelVisibility: CapabilityModelVisibility.Hidden;
  inputSchema: z.ZodType;
  inputIntentSchema?: never;
  result?: never;
};

export type CapabilityToolDefinition =
  | ContractedCapabilityToolDefinition
  | UncontractedCapabilityToolDefinition;

export type CapabilityToolCatalog = {
  protocolVersion: string;
  tools: Array<z.infer<typeof capabilityDescriptorSchema>>;
};

export const toolInvokeOutputSchema = z.strictObject({
  content: z.string().optional(),
  effects: z.array(z.strictObject({
    contentType: z.string().optional(),
    durability: z.string().optional(),
    effect: z.string(),
    filename: z.string().optional(),
    id: z.string().optional(),
    objectType: z.string(),
    path: z.string().optional(),
    summary: z.string().optional(),
    url: z.string().optional(),
    visibility: z.string().optional(),
  })).optional(),
  errorCode: z.string().optional(),
  failureStage: z.string().optional(),
  isError: z.boolean().optional(),
  message: z.string().optional(),
  outcome: z.enum(toolInvokeOutcomes).optional(),
  provider: z.string(),
  result: z.unknown(),
  retryable: z.boolean().optional(),
  safeRetry: z.boolean().optional(),
  selectedBackend: z.string(),
  status: z.string().optional(),
  toolName: z.string(),
});

export function buildCapabilityCatalog(
  protocolVersion: string,
  definitions: CapabilityToolDefinition[],
): CapabilityToolCatalog {
  return { protocolVersion, tools: definitions.map(buildCapabilityDescriptor) };
}

export function buildCapabilityDescriptor(
  definition: CapabilityToolDefinition,
): z.infer<typeof capabilityDescriptorSchema> {
  const modelVisibility = definition.modelVisibility ?? CapabilityModelVisibility.Visible;
  const outputSchema = z.toJSONSchema(definition.result?.schema ?? toolInvokeOutputSchema);
  return capabilityDescriptorSchema.parse({
    name: definition.name,
    canonicalName: definition.name,
    namespace: definition.namespace,
    namespaceSummary: capabilityNamespaceSummaries[definition.namespace],
    answeredBy: definition.answeredBy,
    modelName: definition.name,
    modelVisibility,
    modelVisible: modelVisibility === CapabilityModelVisibility.Visible,
    description: definition.description,
    version: definition.version,
    privacyClass: definition.privacyClass,
    estimatedLatency: definition.estimatedLatency,
    requiresUserPresence: definition.requiresUserPresence ?? false,
    requiresRequesterDevice: definition.requiresRequesterDevice,
    approvalScope: definition.approvalScope,
    worksOffline: definition.worksOffline ?? false,
    inputSchema: z.toJSONSchema(definition.inputSchema),
    inputIntentSchema: definition.inputIntentSchema === undefined
      ? undefined
      : z.toJSONSchema(definition.inputIntentSchema),
    outputSchema,
    inputSchemaStrict: true,
    outputSchemaStrict: true,
    resultContract: definition.result === undefined
      ? undefined
      : {
        schema: outputSchema,
        effects: definition.result.effects,
        evidenceCondition: definition.result.evidenceCondition,
      },
    policyResource: definition.policyResource,
    sideEffectClass: definition.sideEffect,
    sideEffect: definition.sideEffect,
    requiresApproval: definition.requiresApproval,
    completionEvidence: definition.completionEvidence,
    availability: { state: CapabilityAvailabilityState.Available },
    idempotency: definition.idempotency ?? { supported: false, required: false, scope: 'operation' },
  });
}
