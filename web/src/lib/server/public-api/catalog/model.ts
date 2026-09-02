import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const attentionTriageInputSchema = z.strictObject({
  createdAt: z.string().optional(),
  denialCode: z.string().optional(),
  error: z.string().optional(),
  expiresAt: z.string().optional(),
  jobID: z.string(),
  lastAttentionAt: z.string().optional(),
  privacyClass: z.string().optional(),
  status: z.string(),
  toolName: z.string(),
  updatedAt: z.string().optional(),
  watchAttemptCount: z.int().optional(),
});

const embeddingCreateInputSchema = z.strictObject({
  executionMode: z.string().optional(),
  input: z.unknown(),
  inputType: z.string().optional(),
  model: z.string().optional(),
  outputDimensions: z.int().optional(),
  provider: z.string().optional(),
  task: z.string().optional(),
  title: z.string().optional(),
});

const llmStructuredInputSchema = z.strictObject({
  accelerator: z.string().optional(),
  enableResponseHealing: z.boolean().optional(),
  executionMode: z.string().optional(),
  messages: z.array(z.strictObject({
    content: z.string().optional(),
    parts: z.array(z.strictObject({
      dataBase64: z.string().optional(),
      mimeType: z.string().optional(),
      text: z.string().optional(),
      type: z.string(),
    })).optional(),
    role: z.string(),
  })),
  model: z.string().optional(),
  provider: z.string().optional(),
  requireParameters: z.boolean().optional(),
  structuredOutputSchema: z.strictObject({
    document: z.unknown(),
    isStrictlyEnforced: z.boolean().optional(),
    name: z.string(),
  }),
});

const llmTextInputSchema = z.strictObject({
  accelerator: z.string().optional(),
  enableResponseHealing: z.boolean().optional(),
  executionMode: z.string().optional(),
  messages: z.array(z.strictObject({
    content: z.string().optional(),
    parts: z.array(z.strictObject({
      dataBase64: z.string().optional(),
      mimeType: z.string().optional(),
      text: z.string().optional(),
      type: z.string(),
    })).optional(),
    role: z.string(),
  })),
  model: z.string().optional(),
  provider: z.string().optional(),
  requireParameters: z.boolean().optional(),
});

export const modelToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "attention.triage",
    namespace: "attention",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "model_input",
    policyResource: "tool:attention.triage",
    description: "Classify whether a pending companion job needs remote attention.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: attentionTriageInputSchema,
    sideEffect: CapabilitySideEffect.Computation,
    worksOffline: true,
  },
  {
    name: "embedding_create",
    namespace: "embedding",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "model_input",
    policyResource: "tool:embedding_create",
    description: "Create embeddings with the companion's local embedding model.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: embeddingCreateInputSchema,
    sideEffect: CapabilitySideEffect.Computation,
    worksOffline: true,
  },
  {
    name: "llm_structured",
    namespace: "llm",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "model_input",
    policyResource: "tool:llm_structured",
    description: "Generate schema-constrained output with the companion's local language model.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: llmStructuredInputSchema,
    sideEffect: CapabilitySideEffect.Computation,
    worksOffline: true,
  },
  {
    name: "llm_text",
    namespace: "llm",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "model_input",
    policyResource: "tool:llm_text",
    description: "Generate text with the companion's local language model.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: llmTextInputSchema,
    sideEffect: CapabilitySideEffect.Computation,
    worksOffline: true,
  },
];
