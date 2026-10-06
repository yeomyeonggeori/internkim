import { expect, test } from 'bun:test';
import { ToolRefused } from '../../../src/lib/tool-answer';
import { MemoryReadError, isMemoryAccessDenied } from '../../../src/routes/memory/memory-read-error';
import { RunsReadError, isRunsAccessDenied } from '../../../src/routes/runs/runs-read-error';

test('memory denial follows known transport status rather than error-message wording', () => {
	expect(isMemoryAccessDenied(new MemoryReadError('read refused', 401))).toBe(true);
	expect(isMemoryAccessDenied(new MemoryReadError('read refused', 403))).toBe(true);
	expect(isMemoryAccessDenied(new ToolRefused('circle access refused', 'denied', 403))).toBe(true);
	expect(isMemoryAccessDenied(new MemoryReadError('temporary failure', 503))).toBe(false);
	expect(isMemoryAccessDenied(new Error('403 forbidden'))).toBe(false);
});

test('run read status distinguishes authorization denial from recoverable failure', () => {
	expect(isRunsAccessDenied(new RunsReadError('read refused', 401))).toBe(true);
	expect(isRunsAccessDenied(new RunsReadError('read refused', 403))).toBe(true);
	expect(isRunsAccessDenied(new RunsReadError('temporary failure', 503))).toBe(false);
	expect(isRunsAccessDenied(new Error('401 unauthorized'))).toBe(false);
});
