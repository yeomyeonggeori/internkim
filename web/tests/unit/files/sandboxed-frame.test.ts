import { describe, expect, test } from 'bun:test';
import {
	isolatedContentPolicy,
	isolatedHTMLDocument,
	presentationFrameDocument,
	presentationFrameMessageSchema
} from '../../../src/lib/files/preview/sandboxed-frame';

const policy = `<meta http-equiv="Content-Security-Policy" content="${isolatedContentPolicy}">`;

describe('isolatedHTMLDocument', () => {
	test('places the policy first so it governs everything the page loads', () => {
		expect(isolatedHTMLDocument('<p>hello</p>')).toBe(`${policy}<p>hello</p>`);
	});

	test('keeps a leading doctype first so the page stays in standards mode', () => {
		expect(isolatedHTMLDocument('﻿ <!DOCTYPE html><html>')).toBe(`﻿ <!DOCTYPE html>${policy}<html>`);
	});

	test('allows no network host for any kind of resource', () => {
		expect(isolatedContentPolicy).toStartWith("default-src 'none'");
		expect(isolatedContentPolicy).not.toMatch(/https?:|\*/);
	});
});

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
