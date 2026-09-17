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
  etcBusinessColor: z.string().optional(),
  etcTypeColor: z.string().optional(),
});

const writtenLabelSchema = z.strictObject({
  name: z.string().min(1).max(128).describe('The label as people read it on a task.'),
  color: z.string().max(64).describe('CSS colour the board paints this label in. Omit to let one be derived from the name.').optional(),
});

export const taskVocabularySetInputSchema = z.strictObject({
  businesses: z.array(writtenLabelSchema)
    .describe('The business labels this company runs, replacing the whole list. Read registeredLabels from a task_list result first and send it back with what changes.')
    .optional(),
  types: z.array(writtenLabelSchema)
    .describe('The task type labels this company runs, replacing the whole list.')
    .optional(),
  etcBusinessColor: z.string().max(64).describe('Colour for work carrying no business label.').optional(),
  etcTypeColor: z.string().max(64).describe('Colour for work carrying no type label.').optional(),
});

export const taskVocabularySetInputIntentSchema = taskVocabularySetInputSchema.partial();
