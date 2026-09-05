import { describe, expect, test } from 'bun:test';
import { z } from 'zod';
import { taskSizes } from '../../../src/lib/task/task-sizes';
import {
  taskAddInputSchema,
  taskUpdateInputSchema,
} from '../../../src/lib/server/public-api/catalog/tools';

describe('task classification contracts', () => {
  test('every fixed size is accepted and the model receives its definition', () => {
    const description = z.toJSONSchema(taskAddInputSchema.shape.size).description ?? '';
    for (const definition of taskSizes('en')) {
      expect(taskAddInputSchema.safeParse({ title: 'Sample work', size: definition.name }).success).toBe(true);
      expect(description).toContain(definition.developmentExample);
      expect(description).toContain(definition.otherExample);
      expect(description).toContain(definition.note);
    }
    expect(taskAddInputSchema.safeParse({ title: 'Sample work', size: 'XXXL' }).success).toBe(false);
  });

  test('an empty type clears its value while omission remains a partial update', () => {
    expect(taskAddInputSchema.parse({ title: 'Sample work', size: 'M', type: '' }).type).toBe('');
    expect(taskUpdateInputSchema.parse({ taskHint: 'task-1', type: '' }).type).toBe('');
    expect(taskUpdateInputSchema.parse({ taskHint: 'task-1', size: 'S' })).not.toHaveProperty('type');
  });

});
