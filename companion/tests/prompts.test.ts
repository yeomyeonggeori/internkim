import { describe, expect, test } from 'vitest';
import { approvalResponse, confirmResponse, inputResponse, normalizePromptRequest, promptTitle } from '../src/lib/prompts';

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

	test('normalizes approval payloads', () => {
		const request = normalizePromptRequest({
			requestId: 'prompt-3',
			kind: 'approval',
			message: 'Allow browser work?',
			toolName: 'browser.navigate',
			capabilityScope: 'browser',
			resourceScope: { kind: 'web_origin', value: 'https://github.com' }
		});

		expect(request.resourceScope?.value).toBe('https://github.com');
		expect(request.capabilityScope).toBe('browser');
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
		expect(approvalResponse(false, 'use another way')).toEqual({
			allowed: false,
			userReason: 'use another way',
			suggestedConstraint: 'use another way'
		});
		expect(approvalResponse(true, '', true)).toEqual({
			allowed: true,
			rememberSession: true
		});
	});

	test('labels prompt cards', () => {
		expect(promptTitle({ requestID: 'prompt-1', kind: 'confirm', message: 'Continue?' })).toBe('Confirmation needed');
		expect(promptTitle({ requestID: 'prompt-2', kind: 'input', message: 'Name?' })).toBe('Input needed');
		expect(promptTitle({ requestID: 'prompt-3', kind: 'approval', message: 'Allow?' })).toBe('Permission needed');
	});
});
