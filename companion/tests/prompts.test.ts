import { describe, expect, test } from 'vitest';
import { confirmResponse, inputResponse, normalizePromptRequest, promptTitle } from '../src/lib/prompts';

describe('prompt bridge payloads', () => {
	test('normalizes Tauri prompt event payloads', () => {
		const request = normalizePromptRequest({
			requestId: 'prompt-1',
			kind: 'confirm',
			message: 'Continue?',
			default: true
		});

		expect(request).toEqual({
			requestID: 'prompt-1',
			kind: 'confirm',
			message: 'Continue?',
			default: true
		});
	});

	test('rejects unsupported prompt kinds', () => {
		expect(() =>
			normalizePromptRequest({
				requestId: 'prompt-1',
				kind: 'file',
				message: 'Pick a file'
			})
		).toThrow();
	});

	test('creates bridge responses', () => {
		expect(confirmResponse(true)).toEqual({ confirmed: true });
		expect(inputResponse('hello')).toEqual({ text: 'hello' });
	});

	test('labels prompt cards', () => {
		expect(promptTitle({ requestID: 'prompt-1', kind: 'confirm', message: 'Continue?' })).toBe('Confirmation needed');
		expect(promptTitle({ requestID: 'prompt-2', kind: 'input', message: 'Name?' })).toBe('Input needed');
	});
});
