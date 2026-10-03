import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
	approvalReplyExtensionMethod,
	approvalReplyRequest,
	deliveredExtensionMethod,
	deliveredReport,
	deliveryMetaKey,
	deliveryOf,
	messageMetaKey,
	optionChosenIn,
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
};

function blueclawContract(): ClientContract {
	return JSON.parse(readFileSync(contractPath, 'utf8'));
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
			approvalReply: approvalReplyExtensionMethod,
			delivered: deliveredExtensionMethod,
			undelivered: undeliveredExtensionMethod
		};
		expect(relayMethods).toEqual(contract.extensionMethods);
	});

	test('with the same field names in each shape', () => {
		const everyDeliveryField = deliveryOf({
			[deliveryMetaKey]: Object.fromEntries(contract.fields.delivery.map((name) => [name, `a ${name}`]))
		});
		const relayFields: Record<string, string[]> = {
			delivery: Object.keys(everyDeliveryField),
			delivered: Object.keys(deliveredReport('delivery', 'message')),
			undelivered: Object.keys(undeliveredReport('delivery', 'reason')),
			approvalReply: Object.keys(approvalReplyRequest('session', 'tool call', 'reply')),
			approvalReplyAnswer: contract.fields.approvalReplyAnswer.filter(
				(name) => optionChosenIn({ [name]: 'approve_once' }) === 'approve_once'
			)
		};
		expect(relayFields).toEqual(contract.fields);
	});
});
