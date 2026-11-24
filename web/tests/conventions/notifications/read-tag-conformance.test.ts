import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { readTagKey as sharedReadTagKey } from '../../../../supabase/functions/_shared/push-vocabulary.ts';
import { readTagKey as macReadTagKey } from '../../../src/lib/native-shell/read-notifications';

function sourceOf(path: string): string {
	return readFileSync(new URL(path, import.meta.url), 'utf8');
}

describe('the field a push names a read conversation in', () => {
	test('is the one the Mac app reads', () => {
		expect(macReadTagKey).toBe(sharedReadTagKey);
	});

	test('is the one the iPhone app reads', () => {
		expect(sourceOf('../../../src-tauri/gen/apple/Shared/SilentPush.swift')).toContain(`userInfo["${sharedReadTagKey}"]`);
	});

	test('is the one the Android app reads', () => {
		expect(
			sourceOf('../../../sender-notification/android/src/main/java/app/intern/kim/notification/ReadNotificationClearing.java')
		).toContain(`readTagKey = "${sharedReadTagKey}"`);
	});
});
