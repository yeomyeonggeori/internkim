import { expect, test } from 'bun:test';
import {
	aCompanyPlane,
	admindArgumentsForPlane,
	blueclawArgumentsForPlane,
	capabilitydArgumentsForPlane
} from './a-company-plane';

test('the plane starts every daemon with the current company-box contract', () => {
	const argumentsForCapabilityd = capabilitydArgumentsForPlane({
		socketPath: '/tmp/capability.sock',
		openRouterKeyPath: '/tmp/openrouter-key',
		blueclawURL: 'http://127.0.0.1:8080',
		admindURL: 'http://127.0.0.1:8081',
		chatdEndpoint: 'http://127.0.0.1:8082',
		chatdPlatform: 'buzz',
		deviceBrowserPath: '/usr/bin/moli',
		deviceBrowserStateDirectory: '/tmp/device-browsers',
		relayURL: 'http://127.0.0.1:8083',
		admindSocketPath: '/tmp/admind.sock'
	});

	expect(argumentsForCapabilityd).toContain('--device-browser');
	expect(argumentsForCapabilityd).toContain('--relay-url');
	expect(argumentsForCapabilityd).not.toContain('--device-browser-cdp');

	const argumentsForBlueclaw = blueclawArgumentsForPlane({
		runtimeConfigurationPath: '/tmp/runtime.json',
		policyPath: '/tmp/policy.json',
		acpSocketPath: '/tmp/blueclaw-acp.sock',
		inbound: 'acp'
	});
	expect(argumentsForBlueclaw).toContain('-acp-socket');

	const argumentsForAdmind = admindArgumentsForPlane({
		listenAddress: '127.0.0.1:8081',
		capabilitySocketPath: '/tmp/capability.sock',
		chatdEndpoint: 'http://127.0.0.1:8082',
		chatdPlatform: 'buzz',
		blueclawURL: 'http://127.0.0.1:8080',
		blueclawPolicyPath: '/tmp/policy.json',
		buzzKeySeedPath: '/tmp/buzz-key-seed',
		buzzDatabaseURLPath: '/tmp/buzz-database-url',
		buzzRelayKeyPath: '/tmp/buzz-relay-key',
		buzzAdminCommandPath: '/usr/bin/buzz-admin',
		buzzRelayURL: 'ws://127.0.0.1:8084',
		buzzAccountLinksPath: '/tmp/buzz-account-links.json',
		siteScaffoldPath: '/tmp/site-scaffold',
		centralPlaneAppURL: 'https://app.example.test',
		centralPlaneAgentKeyPath: '/tmp/agent-key',
		centralPlaneProjectURL: 'https://project.example.test',
		centralPlanePublishableKey: 'publishable-key',
		listenSocketPath: '/tmp/admind.sock',
		stateDirectory: '/tmp/state',
		databasePath: '/tmp/internkim.sqlite'
	});
	expect(argumentsForAdmind).toContain('-buzz-database-url-path');
	expect(argumentsForAdmind).toContain('-buzz-relay-key-path');
	expect(argumentsForAdmind).toContain('-buzz-account-links');
});

test('a plane told two different messengers refuses to start', async () => {
	const refusal = aCompanyPlane({ messengerPlatform: 'buzz', disagreeAbout: 'capabilityd' });
	await expect(refusal).rejects.toThrow(/disagrees with itself/);
}, 120_000);
