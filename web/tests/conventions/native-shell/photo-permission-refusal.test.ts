import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { photoPermissionRefusalReason } from '$lib/native-shell/media-save';

const rust = readFileSync('../.dependency/internkim-app/src-tauri/plugins/media-save/src/lib.rs', 'utf8');

function rustRejectionCode(): string {
	const match = rust.match(/const PHOTO_PERMISSION_REFUSED: &str = "(\w+)";/);
	if (!match) throw new Error('media-save names no rejection code for a refused photo permission');
	return match[1];
}

describe('a refused photo permission reaches the page as a reason it reads', () => {
	test('the Swift plugin rejects with the code the Rust plugin reads', () => {
		const swift = readFileSync('../.dependency/internkim-app/src-tauri/plugins/media-save/ios/Sources/MediaSavePlugin.swift', 'utf8');
		expect(swift).toContain(`private let photoPermissionRefused = "${rustRejectionCode()}"`);
	});

	test('the Kotlin plugin rejects with the code the Rust plugin reads', () => {
		const kotlin = readFileSync('../.dependency/internkim-app/src-tauri/gen/android/app/src/main/java/app/intern/kim/MediaSavePlugin.kt', 'utf8');
		expect(kotlin).toContain(`private const val PHOTO_PERMISSION_REFUSED = "${rustRejectionCode()}"`);
	});

	test('the Rust plugin names the reason the page reads', () => {
		expect(rust).toContain(`#[serde(rename = "${photoPermissionRefusalReason}")]\n    PhotoPermission,`);
	});
});
