import { expect, test } from 'bun:test';
import {
	aCompanyPlane,
	admindArgumentsForPlane,
	blueclawArgumentsForPlane,
	capabilitydArgumentsForPlane,
	embeddingServerCommandForPlane
} from './a-company-plane';

test('the plane starts every daemon with the current company-box contract', () => {
	const argumentsForCapabilityd = capabilitydArgumentsForPlane({
		socketPath: '/tmp/capability.sock',
		blueclawWorkspacePath: '/tmp/workspace',
		openRouterKeyPath: '/tmp/openrouter-key',
		blueclawURL: 'http://127.0.0.1:8080',
		embeddingServerURL: 'http://127.0.0.1:8083',
		admindURL: 'http://127.0.0.1:8081',
		chatdEndpoint: 'http://127.0.0.1:8082',
		chatdPlatform: 'buzz',
		deviceBrowserPath: '/usr/bin/moli',
		deviceBrowserStateDirectory: '/tmp/device-browsers',
		fileReadPythonPath: 'python3',
		admindSocketPath: '/tmp/admind.sock'
	});

	expect(argumentsForCapabilityd).toContain('--device-browser');
	expect(argumentsForCapabilityd).toContain('--file-read-python');
	expect(argumentsForCapabilityd).toContain('--embedding-url');
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
		blueclawWorkspacePath: '/tmp/workspace',
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
		buzzRelayPublicURL: 'wss://acme.app.example.test',
		buzzAccountLinksPath: '/tmp/buzz-account-links.json',
		centralPlaneAppURL: 'https://app.example.test',
		centralPlaneAgentKeyPath: '/tmp/agent-key',
		blueclawAssertionKeyPath: '/tmp/blueclaw-assertion-key',
		centralPlaneProjectURL: 'https://project.example.test',
		centralPlanePublishableKey: 'publishable-key',
		listenSocketPath: '/tmp/admind.sock',
		stateDirectory: '/tmp/state',
		databasePath: '/tmp/internkim.sqlite',
		agentProfilePicturePath: '/tmp/agent-picture.png'
	});
	expect(argumentsForAdmind).toContain('-buzz-database-url-path');
	expect(argumentsForAdmind).toContain('-buzz-relay-key-path');
	expect(argumentsForAdmind).toContain('-buzz-account-links');
	expect(argumentsForAdmind).toContain('-buzz-relay-public-url');
	expect(argumentsForAdmind).toContain('-blueclaw-assertion-key');
	expect(argumentsForAdmind).toContain('-agent-profile-picture');
	expect(argumentsForAdmind).toContain('/tmp/agent-picture.png');
});

test('the plane starts the embedding server the way the package does, on its own model and port', () => {
	const command = embeddingServerCommandForPlane('/plane/llama.cpp', '/plane/models/model.gguf', 18123);

	expect(command[0]).toBe('/plane/llama.cpp/llama-server');
	expect(command[command.indexOf('-m') + 1]).toBe('/plane/models/model.gguf');
	expect(command[command.indexOf('--port') + 1]).toBe('18123');
	expect(command).toContain('--embeddings');
	expect(command[command.indexOf('--pooling') + 1]).toBe('mean');
	expect(command[command.indexOf('--device') + 1]).toBe('none');
});

test('a plane told two different messengers refuses to start', async () => {
	const refusal = aCompanyPlane({ messengerPlatform: 'buzz', disagreeAbout: 'capabilityd' });
	await expect(refusal).rejects.toThrow(/disagrees with itself/);
}, 120_000);
