import { describe, expect, test } from 'bun:test';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

import {
	buildPluginToolProvenanceReport,
	parseToolReferences,
	readPluginToolProvenanceReport
} from '../../../scripts/plugin-tool-provenance';
import type { OwnershipDescriptor } from '../../../scripts/plugin-tool-provenance';
import { CapabilityAnsweredBy } from '../../../src/lib/server/public-api/catalog/protocol';

function descriptor(name: string, answeredBy: CapabilityAnsweredBy): OwnershipDescriptor {
	return { name, answeredBy };
}

describe('plugin tool provenance', () => {
	test('parses references without deciding ownership', () => {
		expect(
			parseToolReferences(
				'---\r\nmetadata:\r\n  kim.intern.tool-references: "event_add shell event_add"\r\n---\r\n' +
					'Body mentions kim.intern.tool-references: ignored.'
			)
		).toEqual(['event_add', 'shell']);
		expect(parseToolReferences('---\nname: no-tools\n---\nBody')).toEqual([]);
		expect(parseToolReferences('Body mentions kim.intern.tool-references: ignored.')).toEqual([]);
	});

	test('rejects malformed or non-string frontmatter metadata', () => {
		expect(() => parseToolReferences('---\nmetadata:\n  kim.intern.tool-references: [\n---\nBody')).toThrow();
		expect(
			() => parseToolReferences('---\nmetadata:\n  kim.intern.tool-references:\n    - event_add\n---\nBody')
		).toThrow();
		expect(() => parseToolReferences('---\nmetadata:\n  kim.intern.tool-references: event_add')).toThrow('not closed');
		expect(() => parseToolReferences('---\nmetadata: {}\n---body\nBody')).toThrow('not closed');
	});

	test('separates service, local, and unresolved references', () => {
		const report = buildPluginToolProvenanceReport(
			{
				name: 'fixture',
				version: '1',
				skills: [{
					name: 'fixture',
					document: '---\nmetadata:\n  kim.intern.tool-references: "event_add browser_open schedule_create"\n---\nBody'
				}]
			},
			[descriptor('event_add', CapabilityAnsweredBy.Record)],
			[descriptor('event_add', CapabilityAnsweredBy.Record), descriptor('browser_open', CapabilityAnsweredBy.Local)],
			0
		);

		expect(report.skills[0]).toMatchObject({
			serviceReferences: ['event_add'],
			localReferences: ['browser_open'],
			unresolvedReferences: ['schedule_create']
		});
		expect(report.summary).toMatchObject({
			referenceCount: 3,
			serviceReferenceCount: 1,
			localReferenceCount: 1,
			unresolvedReferenceCount: 1
		});
	});

	test('measures the checked-in plugin against the full-permission model catalog', async () => {
		const repositoryRoot = join(import.meta.dir, '../../../..');
		const report = await readPluginToolProvenanceReport(repositoryRoot);

		expect(report.plugin.name).toBe('internkim');
		expect(report.catalog.modelVisibleFullPermission).toBeGreaterThan(0);
		expect(report.catalog.modelSchemaBytes).toBeGreaterThan(0);
		expect(report.references.service).toContain('task_add');
		expect(report.skills.length).toBeGreaterThan(0);
	});

	test('fails loudly when the selected plugin is missing a skill document', async () => {
		const repositoryRoot = await mkdtemp(join(tmpdir(), 'plugin-provenance-'));
		try {
			await mkdir(join(repositoryRoot, '.dependency', 'unrelated'), { recursive: true });
			await writeFile(join(repositoryRoot, '.dependency', 'unrelated', 'plugin.json'), '{broken');
			await mkdir(join(repositoryRoot, '.dependency', 'internkim', 'skills', 'fixture'), { recursive: true });
			await writeFile(join(repositoryRoot, '.dependency', 'internkim', 'plugin.json'), '{"name":"internkim","version":"1"}');
			await expect(readPluginToolProvenanceReport(repositoryRoot)).rejects.toThrow('SKILL.md');
		} finally {
			await rm(repositoryRoot, { recursive: true, force: true });
		}
	});

	test('ignores malformed unrelated manifests', async () => {
		const repositoryRoot = await mkdtemp(join(tmpdir(), 'plugin-provenance-'));
		try {
			await mkdir(join(repositoryRoot, '.dependency', 'unrelated'), { recursive: true });
			await writeFile(join(repositoryRoot, '.dependency', 'unrelated', 'plugin.json'), '{"name":"other"}');
			await mkdir(join(repositoryRoot, '.dependency', 'internkim', 'skills', 'fixture'), { recursive: true });
			await writeFile(join(repositoryRoot, '.dependency', 'internkim', 'plugin.json'), '{"name":"internkim","version":"1"}');
			await writeFile(join(repositoryRoot, '.dependency', 'internkim', 'skills', 'fixture', 'SKILL.md'), '---\nname: fixture\n---\nBody');
			const report = await readPluginToolProvenanceReport(repositoryRoot);
			expect(report.plugin.name).toBe('internkim');
			expect(report.plugin.skillCount).toBe(1);
		} finally {
			await rm(repositoryRoot, { recursive: true, force: true });
		}
	});
});
