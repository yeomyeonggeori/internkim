import { mkdir, readFile, writeFile, unlink } from 'node:fs/promises';
import { closeSync, openSync } from 'node:fs';
import { join, resolve } from 'node:path';

type Child = ReturnType<typeof Bun.spawn>;
type Part = { name: string; logPath: string; child?: Child };

const repositoryRoot = resolve(import.meta.dir, '..');
const parts: Part[] = [];
const failureTailLineCount = 40;
let appPIDPath = '';
let fleetConfigPath = '';
let remoteRelayPID = '';

function argument(name: string): string {
	const index = process.argv.indexOf(name);
	const value = index >= 0 ? process.argv[index + 1] : '';
	if (!value) throw new Error(`${name} is required`);
	return value;
}

async function freePort(): Promise<number> {
	const server = Bun.serve({ port: 0, fetch: () => new Response() });
	const port = server.port;
	server.stop(true);
	if (!port) throw new Error('could not reserve a local port');
	return port;
}

function environmentFrom(document: string): Record<string, string> {
	return Object.fromEntries(
		document
			.split('\n')
			.map((line) => line.trim())
			.filter((line) => line && !line.startsWith('#'))
			.map((line) => {
				const separator = line.indexOf('=');
				const value = line.slice(separator + 1);
				const isQuoted = (value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"));
				return [line.slice(0, separator), isQuoted ? value.slice(1, -1) : value];
			})
	);
}

function start(name: string, command: string[], environment: Record<string, string>, logPath: string, cwd = repositoryRoot): Child {
	const descriptor = openSync(logPath, 'w');
	const child = Bun.spawn(command, {
		cwd,
		env: { ...process.env, ...environment },
		stdout: descriptor,
		stderr: descriptor
	});
	closeSync(descriptor);
	parts.push({ name, logPath, child });
	return child;
}

function stateOf(part: Part): string {
	if (!part.child) return 'runs in the Local Fleet';
	if (part.child.exitCode === null) return 'is still running';
	return `exited with code ${part.child.exitCode}`;
}

async function endOfLog(logPath: string): Promise<string> {
	const written = await readFile(logPath, 'utf8').catch(() => '');
	const lines = written.split('\n').filter((line) => line.trim());
	return lines.slice(-failureTailLineCount).join('\n');
}

async function reportWhatEachPartDid(): Promise<void> {
	for (const part of parts) {
		console.error(`${part.name} ${stateOf(part)}; its output is at ${part.logPath}`);
		if (!part.child || part.child.exitCode === null) continue;
		const ending = await endOfLog(part.logPath);
		if (ending) console.error(ending);
	}
}

function shellQuote(value: string): string {
	return `'${value.replaceAll("'", "'\\''")}'`;
}

async function commandOutput(command: string[], cwd = repositoryRoot): Promise<string> {
	const processValue = Bun.spawn(command, { cwd, stdout: 'pipe', stderr: 'pipe' });
	const output = await new Response(processValue.stdout).text();
	const errorText = await new Response(processValue.stderr).text();
	if (await processValue.exited !== 0) throw new Error(`${command[0]} failed: ${errorText.trim()}`);
	return output.trim();
}

async function localPlaneSigningKey(jwtSecret: string): Promise<string> {
	return commandOutput([
		'sh',
		'-c',
		'. "$1" && signing_key_of_secret "$2"',
		'sh',
		join(repositoryRoot, 'web/scripts/local-plane-signing-key.sh'),
		jwtSecret
	]);
}

async function waitForHTTP(url: string, expectedStatus?: number, timeoutMilliseconds = 30_000): Promise<void> {
	const deadline = Date.now() + timeoutMilliseconds;
	while (Date.now() < deadline) {
		const response = await fetch(url, { signal: AbortSignal.timeout(2000) }).catch(() => null);
		if (response && (expectedStatus === undefined || response.status === expectedStatus)) return;
		await Bun.sleep(250);
	}
	throw new Error(`timed out waiting for ${url}`);
}

async function stopChildren(): Promise<void> {
	if (remoteRelayPID && fleetConfigPath) {
		await commandOutput([
			'./internkim',
			'lab',
			'vm-ssh',
			'--config',
			fleetConfigPath,
			'--',
			`sudo kill ${shellQuote(remoteRelayPID)} 2>/dev/null || true`
		]);
	}
	const spawned = parts.flatMap((part) => (part.child ? [part.child] : []));
	for (const child of [...spawned].reverse()) child.kill();
	for (const child of spawned) await child.exited.catch(() => 0);
	if (appPIDPath) await unlink(appPIDPath).catch(() => undefined);
}

async function main(): Promise<number> {
	const stateRoot = resolve(argument('--state-root'));
	const appPort = argument('--app-port');
	const adminPort = argument('--admin-port');
	const chatdURL = argument('--chatd-url');
	fleetConfigPath = resolve(argument('--config'));
	const gatewayPort = await freePort();
	const gatewayURL = `http://127.0.0.1:${gatewayPort}`;
	const centralPlane = environmentFrom(await readFile(join(stateRoot, 'central-plane.env'), 'utf8'));
	const companyID = centralPlane.CENTRAL_PLANE_COMPANY_ID;
	const gatewayAdminToken = crypto.randomUUID();
	const gatewayServerKey = crypto.randomUUID();
	const gatewayLog = join(stateRoot, 'personal-settings-gateway.log');
	const viteLog = join(stateRoot, 'personal-settings-vite.log');
	const sshLog = join(stateRoot, 'personal-settings-ssh.log');
	const gatewayConfigurationPath = join(stateRoot, 'personal-settings-gateway.jsonc');

	await mkdir(stateRoot, { recursive: true });
	const canonicalGatewayConfiguration = JSON.parse(
		await readFile(join(repositoryRoot, 'workers/connection-gateway/wrangler.jsonc'), 'utf8')
	);
	await writeFile(
		gatewayConfigurationPath,
		JSON.stringify(
			{
				...canonicalGatewayConfiguration,
				main: join(repositoryRoot, 'workers/connection-gateway/src/index.ts'),
				vars: {
					SUPABASE_URL: centralPlane.CENTRAL_PLANE_PROJECT_URL,
					SUPABASE_PUBLISHABLE_KEY: centralPlane.CENTRAL_PLANE_PUBLISHABLE_KEY,
					GATEWAY_ADMIN_TOKEN: gatewayAdminToken
				}
			},
			null,
			2
		),
		{ mode: 0o600 }
	);
	const supabaseEnvironment = environmentFrom(await commandOutput(['supabase', 'status', '-o', 'env']));
	const planeSecretKey = supabaseEnvironment.SECRET_KEY;
	const planeJWTSecret = supabaseEnvironment.JWT_SECRET;
	if (!planeSecretKey || !planeJWTSecret) throw new Error('the local plane named no SECRET_KEY or JWT_SECRET, so the company app cannot reach it');
	const planeSigningKey = await localPlaneSigningKey(planeJWTSecret);

	start(
		'the connection gateway',
		['bunx', 'wrangler', 'dev', '--local', '--ip', '127.0.0.1', '--port', String(gatewayPort), '--inspector-port', '0', '--persist-to', join(stateRoot, 'personal-settings-gateway-state'), '--config', gatewayConfigurationPath],
		{},
		gatewayLog,
		join(repositoryRoot, 'workers/connection-gateway')
	);
	await waitForHTTP(`${gatewayURL}/missing`, 404);
	const registered = await fetch(`${gatewayURL}/company/${encodeURIComponent(companyID)}/server-key`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${gatewayAdminToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ serverKey: gatewayServerKey })
	});
	if (!registered.ok) throw new Error(`gateway server-key registration failed with ${registered.status}`);

	const vmAddressProcess = Bun.spawn(['./internkim', 'lab', 'vm-ip', '--config', fleetConfigPath], { stdout: 'pipe', stderr: 'pipe' });
	const vmAddress = (await new Response(vmAddressProcess.stdout).text()).trim();
	if (!vmAddress) throw new Error('the Local Fleet VM has no address');
	const sshpass = join(repositoryRoot, 'bin/sshpass');
	const tunnel = start(
		'the Local Fleet socket tunnel',
		[
			sshpass,
			'-p',
			'admin',
			'ssh',
			'-N',
			'-o',
			'StrictHostKeyChecking=no',
			'-o',
			'UserKnownHostsFile=/dev/null',
			'-o',
			'ExitOnForwardFailure=yes',
			'-R',
			`${gatewayPort}:127.0.0.1:${gatewayPort}`,
			`admin@${vmAddress}`
		],
		{},
		sshLog
	);
	await Bun.sleep(1000);
	if (tunnel.exitCode !== null) throw new Error('the Local Fleet socket tunnel exited');

	await commandOutput([
		'./internkim', 'lab', 'vm-ssh', '--config', fleetConfigPath, '--',
		`sudo sh -c ${shellQuote('cd /mnt/shared/workspace/host/relay && bun install --frozen-lockfile')}`
	]);

	const oldPIDPath = join(stateRoot, 'central-plane-app.pid');
	const oldPID = Number((await readFile(oldPIDPath, 'utf8').catch(() => '')).trim());
	if (oldPID) {
		try {
			process.kill(oldPID);
		} catch {}
	}
	const app = start(
		'the company app',
		['bunx', 'vite', 'dev', '--host', '127.0.0.1', '--port', appPort, '--strictPort'],
		{
			SUPABASE_URL: centralPlane.CENTRAL_PLANE_PROJECT_URL,
			SUPABASE_PUBLISHABLE_KEY: centralPlane.CENTRAL_PLANE_PUBLISHABLE_KEY,
			SUPABASE_SECRET_KEY: planeSecretKey,
			SUPABASE_JWT_SIGNING_KEY: planeSigningKey,
			VITE_ADMIND_TARGET: `http://127.0.0.1:${adminPort}`,
			GATEWAY_URL: gatewayURL,
			GATEWAY_ADMIN_TOKEN: gatewayAdminToken
		},
		viteLog,
		join(repositoryRoot, 'web')
	);
	appPIDPath = oldPIDPath;
	await writeFile(appPIDPath, String(app.pid));
	await waitForHTTP(`http://127.0.0.1:${appPort}/api/agent/company`);

	const remoteRelayLog = `/mnt/shared/workspace/${stateRoot.slice(repositoryRoot.length + 1)}/personal-settings-relay.log`;
	const remoteRelayState = `/mnt/shared/workspace/${stateRoot.slice(repositoryRoot.length + 1)}/personal-settings-relay-state`;
	const remoteRelayCommand = [
		'mkdir -p', shellQuote(remoteRelayState), '&&',
		'nohup env',
		`SUPABASE_URL=${shellQuote(centralPlane.CENTRAL_PLANE_PROJECT_URL)}`,
		`SUPABASE_PUBLISHABLE_KEY=${shellQuote(centralPlane.CENTRAL_PLANE_PUBLISHABLE_KEY)}`,
		`INTERNKIM_APP_URL=${shellQuote(`http://127.0.0.1:${appPort}`)}`,
		'MESSENGER_PLATFORM=buzz',
		`CHATD_BASE_URL=${shellQuote(chatdURL)}`,
		'AGENT_API_KEY_PATH=/root/.internkim/secrets/central-plane-agent-key',
		`GATEWAY_URL=${shellQuote(gatewayURL)}`,
		`GATEWAY_SERVER_KEY=${shellQuote(gatewayServerKey)}`,
		`RELAY_STATE_DIR=${shellQuote(remoteRelayState)}`,
		`bun run /mnt/shared/workspace/host/relay/relay.ts > ${shellQuote(remoteRelayLog)} 2>&1 < /dev/null & echo $!`
	].join(' ');
	remoteRelayPID = await commandOutput([
		'./internkim', 'lab', 'vm-ssh', '--config', fleetConfigPath, '--',
		`sudo sh -c ${shellQuote(remoteRelayCommand)}`
	]);
	if (!/^\d+$/.test(remoteRelayPID)) throw new Error('the VM relay did not return a process ID');
	parts.push({ name: 'the Local Fleet relay', logPath: join(stateRoot, 'personal-settings-relay.log') });
	const relayDeadline = Date.now() + 30_000;
	while (Date.now() < relayDeadline) {
		const relayLogText = await commandOutput([
			'./internkim', 'lab', 'vm-ssh', '--config', fleetConfigPath, '--',
			`sudo grep -F 'gateway connected for company' ${shellQuote(remoteRelayLog)} 2>/dev/null || true`
		]);
		if (relayLogText) break;
		await Bun.sleep(250);
	}
	if (Date.now() >= relayDeadline) throw new Error('timed out waiting for the VM relay gateway connection');

	const tests = start('the browser suite', ['bun', 'run', 'test:e2e:local-fleet'], { PLAYWRIGHT_BASE_URL: `http://127.0.0.1:${appPort}`, PLAYWRIGHT_START_WEB_SERVER: '0' }, join(stateRoot, 'personal-settings-playwright.log'), join(repositoryRoot, 'web'));
	return await tests.exited;
}

try {
	const result = await main();
	process.exitCode = result;
} catch (failure) {
	console.error(failure instanceof Error ? failure.message : String(failure));
	process.exitCode = 1;
} finally {
	if (process.exitCode) await reportWhatEachPartDid();
	await stopChildren();
}
