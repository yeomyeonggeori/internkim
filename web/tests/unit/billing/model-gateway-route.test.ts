import { describe, expect, test } from 'bun:test';
import { modelGatewayRoute } from '../../../../supabase/functions/_shared/model-gateway-route.ts';

describe('which OpenRouter route a gateway request reaches', () => {
	test('a chat and a decision are charged, the model list is not', () => {
		expect(modelGatewayRoute('POST', '/model-gateway/api/v1/chat/completions')).toEqual({
			upstreamPath: '/api/v1/chat/completions',
			isCharged: true
		});
		expect(modelGatewayRoute('POST', '/functions/v1/model-gateway/api/alpha/decisions')?.isCharged).toBe(true);
		expect(modelGatewayRoute('GET', '/model-gateway/api/v1/models/')?.isCharged).toBe(false);
	});

	test('anything else OpenRouter answers is not passed through', () => {
		expect(modelGatewayRoute('POST', '/model-gateway')).toBeNull();
		expect(modelGatewayRoute('POST', '/model-gateway/api/v1/keys')).toBeNull();
		expect(modelGatewayRoute('GET', '/model-gateway/api/v1/chat/completions')).toBeNull();
	});
});
