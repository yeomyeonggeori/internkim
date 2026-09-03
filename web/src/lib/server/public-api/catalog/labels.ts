import { z } from 'zod';

export const registeredLabelSchema = z.strictObject({
  name: z.string(),
  color: z.string().optional(),
});

export const taskLabelVocabularySchema = z.strictObject({
  businesses: z.array(registeredLabelSchema),
  types: z.array(registeredLabelSchema),
  sizes: z.array(z.string()),
  statuses: z.array(z.string()),
});
