import { describe, expect, test } from 'bun:test';
import {
	presentationFrameDocument,
	presentationFrameMessageSchema
} from '../../../src/lib/files/preview/sandboxed-frame';

describe('presentationFrameDocument', () => {
	test('loads the renderer from the given address', () => {
		expect(presentationFrameDocument('https://app.example.com/_app/pptx.js')).toContain(
			'<script src="https://app.example.com/_app/pptx.js"></script>'
		);
	});
});

describe('presentationFrameMessageSchema', () => {
	test('reads the messages the frame sends and nothing else', () => {
		expect(presentationFrameMessageSchema.safeParse({ type: 'rendered', slideCount: 3 }).success).toBeTrue();
		expect(presentationFrameMessageSchema.safeParse({ type: 'navigate', to: '/' }).success).toBeFalse();
		expect(presentationFrameMessageSchema.safeParse('ready').success).toBeFalse();
	});
});
