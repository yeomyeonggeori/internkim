import { describe, expect, test } from 'bun:test';
import {
	presentationFrameDocument,
	wordFrameDocument,
	frameMessageSchema
} from '../../../src/lib/files/preview/sandboxed-frame';

describe('presentationFrameDocument', () => {
	test('loads the renderer from the given address', () => {
		expect(presentationFrameDocument('https://app.example.com/_app/pptx.js')).toContain(
			'<script src="https://app.example.com/_app/pptx.js"></script>'
		);
	});
});

describe('wordFrameDocument', () => {
	test('loads the zip library before the renderer that needs it', () => {
		const frame = wordFrameDocument('https://app.example.com/_app/jszip.js', 'https://app.example.com/_app/docx.js');
		expect(frame.indexOf('_app/jszip.js')).toBeGreaterThan(0);
		expect(frame.indexOf('_app/jszip.js')).toBeLessThan(frame.indexOf('_app/docx.js'));
	});
});

describe('frameMessageSchema', () => {
	test('reads the messages the frame sends and nothing else', () => {
		expect(frameMessageSchema.safeParse({ type: 'rendered', count: 3 }).success).toBeTrue();
		expect(frameMessageSchema.safeParse({ type: 'navigate', to: '/' }).success).toBeFalse();
		expect(frameMessageSchema.safeParse('ready').success).toBeFalse();
	});
});
