import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
	progressOfToolCall,
	startedToolCallKind,
	toolCallStatuses,
	updatedToolCallKind,
	type StartedToolCall,
	type UpdatedToolCall
} from './tool-progress';
import {
	abandonedOptionKey,
	deliveredExtensionMethod,
	deliveredReport,
	deliveryMetaKey,
	deliveryOf,
	handedBackOptionKey,
	handoffClosedExtensionMethod,
	handoffClosedOf,
	handoffEndedExtensionMethod,
	handoffEndedReport,
	messageMetaKey,
	sessionMetaKey,
	undeliveredExtensionMethod,
	undeliveredReport
} from './acp-session';

const contractPath = join(
	import.meta.dir,
	'../../.dependency/blueclaw/internal/acpsession/client_contract.json'
);

type ClientContract = {
	metaKeys: Record<string, string>;
	extensionMethods: Record<string, string>;
	fields: Record<string, string[]>;
	handoffOptions: Record<string, string>;
	toolCalls: {
		startKind: string;
		startFields: string[];
		updateKind: string;
		updateFields: string[];
		statuses: string[];
	};
};

function blueclawContract(): ClientContract {
	return JSON.parse(readFileSync(contractPath, 'utf8'));
}

function valueOfDeliveryField(name: string): string | boolean {
	return name === 'isFinal' ? true : `a ${name}`;
}

function fieldsReadFromAHandoffClosedNotice(): string[] {
	const read = new Set<string>();
	handoffClosedOf(
		new Proxy<Record<string, unknown>>(
			{ handoffID: 'handoff' },
			{
				get: (target, name) => {
					read.add(String(name));
					return Reflect.get(target, name);
				}
			}
		)
	);
	return [...read];
}

function fieldsReadFrom<Call extends StartedToolCall | UpdatedToolCall>(call: Call): string[] {
	const read = new Set<string>();
	progressOfToolCall(
		new Proxy(call, {
			get: (target, name) => {
				read.add(String(name));
				return Reflect.get(target, name);
			}
		})
	);
	return [...read].sort();
}

function sortedFields(fields: Record<string, string[]>): Record<string, string[]> {
	return Object.fromEntries(Object.entries(fields).map(([shape, names]) => [shape, [...names].sort()]));
}

describe('the relay speaks the ACP extension blueclaw declares', () => {
	const contract = blueclawContract();

	test('under the same _meta keys', () => {
		const relayMetaKeys: Record<string, string> = {
			session: sessionMetaKey,
			message: messageMetaKey,
			delivery: deliveryMetaKey
		};
		expect(relayMetaKeys).toEqual(contract.metaKeys);
	});

	test('under the same extension method names', () => {
		const relayMethods: Record<string, string> = {
			delivered: deliveredExtensionMethod,
			handoffClosed: handoffClosedExtensionMethod,
			handoffEnded: handoffEndedExtensionMethod,
			undelivered: undeliveredExtensionMethod
		};
		expect(relayMethods).toEqual(contract.extensionMethods);
	});

	test('with the same field names in each shape', () => {
		const everyDeliveryField = deliveryOf({
			[deliveryMetaKey]: Object.fromEntries(contract.fields.delivery.map((name) => [name, valueOfDeliveryField(name)]))
		});
		const relayFields: Record<string, string[]> = {
			delivery: Object.keys(everyDeliveryField),
			delivered: Object.keys(deliveredReport('delivery', 'message')),
			undelivered: Object.keys(undeliveredReport('delivery', 'reason')),
			handoffClosed: fieldsReadFromAHandoffClosedNotice(),
			handoffEnded: Object.keys(handoffEndedReport('session', 'handoff', handedBackOptionKey))
		};
		expect(sortedFields(relayFields)).toEqual(sortedFields(contract.fields));
	});

	test('ending a hand-over on the option keys the gate offers', () => {
		const relayOptions: Record<string, string> = { handedBack: handedBackOptionKey, abandoned: abandonedOptionKey };
		expect(relayOptions).toEqual(contract.handoffOptions);
	});

	test('with the same tool call shapes', () => {
		const startedCall: StartedToolCall = {
			sessionUpdate: startedToolCallKind,
			toolCallId: 'call',
			title: 'a title',
			status: 'pending'
		};
		const updatedCall: UpdatedToolCall = { sessionUpdate: updatedToolCallKind, toolCallId: 'call', status: 'completed' };
		const relayToolCalls: ClientContract['toolCalls'] = {
			startKind: startedToolCallKind,
			startFields: fieldsReadFrom(startedCall),
			updateKind: updatedToolCallKind,
			updateFields: fieldsReadFrom(updatedCall),
			statuses: [...toolCallStatuses]
		};
		expect(relayToolCalls).toEqual({
			...contract.toolCalls,
			startFields: [...contract.toolCalls.startFields].sort(),
			updateFields: [...contract.toolCalls.updateFields].sort()
		});
	});
});
