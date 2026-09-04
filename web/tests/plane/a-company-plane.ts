// A company plane, brought up the way a company's own box brings it up, with the
// two messengers standing in as recorders.
//
// The bug this exists for was not a logic bug. host/entrypoint.sh started
// capabilityd without --chatd-platform, so every message tool answered "sent"
// while reaching nothing. No unit test can reach that, because the defect is in
// how the processes are started. So this starts them.

import { SQL } from 'bun';
import { existsSync, mkdirSync, openSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import {
	addMember,
	claimMemberFor,
	issueAgentKey,
	provisionCompany
} from '../../src/lib/server/control-plane';
import {
	aConnectorNobodyRuns,
	aMessengerNobodyRuns,
	type ARecordingMessenger
} from './a-messenger-nobody-runs';
import { aModelNobodyPaysFor, type AModelNobodyPaysFor } from './a-model-nobody-pays-for';
import { theArgumentsThatStart } from '../support/the-entrypoint';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

export type ACompanyPlane = {
	runIdentifier: string;
	companyID: string;
	admin: SupabaseClient;
	people: { email: string; name: string; memberID: string }[];
	agentAPIKey: string;
	admindURL: string;
	blueclawURL: string;
	requesterSocketPath: string;
	capabilitySocketPath: string;
	blueclawACPSocketPath: string;
	relayInboundURL: string;
	runtimeConfigurationPath: string;
	connector: ARecordingMessenger;
	messenger: ARecordingMessenger;
	model: AModelNobodyPaysFor;
	messengerPlatform: string;
	stop: () => Promise<void>;
};

type PlaneRequest = {
	/** The messenger this company runs. Every process is told this one name. */
	messengerPlatform?: string;
	/** Tell one process a different name, to prove the plane refuses to start. */
	disagreeAbout?: 'capabilityd' | 'admind';
	/** Which path admits an inbound message. The relay starts either way. */
	inbound?: 'connectors' | 'acp';
	keepRunDirectory?: boolean;
};

function environmentValue(name: string): string {
	const value = process.env[name];
	if (!value) throw new Error(`${name} is not set; run this through tools/company-plane`);
	return value;
}

// A company box holds no personal access token: one here is a second path to the
// record that skips approval, requester identity and POSIX isolation. Whoever
// runs this has one in their own shell, and the box is not their shell.
function theBoxEnvironment(): Record<string, string | undefined> {
	const environment = { ...process.env };
	delete environment.INTERNKIM_TOKEN;
	return environment;
}

async function aFreePort(): Promise<number> {
	const server = Bun.serve({ port: 0, fetch: () => new Response('') });
	const port = server.port;
	server.stop(true);
	// A server that bound nothing has no port to hand out, and a plane built on a
	// port nobody holds fails later and further away.
	if (port === undefined) throw new Error('the port probe bound no port');
	return port;
}

// Somebody joins by claiming the seat their address was invited to, and that is
// what turns them from pending into active. Writing user_id by hand leaves them
// on the roster and unable to act, which the public API answers as "not active
// member" — so the sandbox arrives the way a person does.
async function arrive(admin: SupabaseClient, email: string): Promise<void> {
	const account = await admin.auth.admin.createUser({
		email,
		password: 'seed-password',
		email_confirm: true
	});
	if (account.error) throw new Error(`${email}: ${account.error.message}`);
	const claimed = await claimMemberFor(admin, account.data.user.id, email);
	if (!claimed) throw new Error(`${email} has no seat to claim`);
}

// blueclaw's record lives in the postgres the local stack already runs, so the
// sandbox needs no container of its own. The run's own database is created on the
// way in and dropped on the way out; nothing else in there is touched.
function databaseURLFor(databaseName: string): string {
	const base = new URL(environmentValue('COMPANY_PLANE_DB_URL'));
	base.pathname = `/${databaseName}`;
	return base.toString();
}

// Bun speaks postgres, so this needs no psql on the developer's machine — one
// fewer thing to install before the gate runs.
async function runPostgres(statement: string): Promise<void> {
	const client = new SQL(environmentValue('COMPANY_PLANE_DB_URL'));
	try {
		await client.unsafe(statement);
	} finally {
		await client.close();
	}
}

// A process that fails to start has already said why; the sandbox keeps that
// where a reader can find it rather than letting it scroll past the scenarios.
function logsTo(path: string): { stdout: number; stderr: number } {
	const descriptor = openSync(path, 'a');
	return { stdout: descriptor, stderr: descriptor };
}

async function untilReady(what: string, ready: () => Promise<boolean>, seconds = 30): Promise<void> {
	for (let attempt = 0; attempt < seconds * 4; attempt += 1) {
		if (await ready()) return;
		await Bun.sleep(250);
	}
	throw new Error(`${what} never became ready`);
}

// Every process that can name a messenger is told the same one, and the plane
// says so before it hands itself to a test. A plane that disagrees with itself
// is the shape of the bug, so it is refused here rather than discovered later in
// a message that went somewhere nobody reads.
function refuseAPlaneThatDisagreesWithItself(named: Record<string, string>): void {
	const distinct = [...new Set(Object.values(named))];
	if (distinct.length === 1) return;
	const table = Object.entries(named)
		.map(([process, platform]) => `  ${process.padEnd(14)} ${platform || '(told nothing)'}`)
		.join('\n');
	throw new Error(
		`this plane disagrees with itself about which messenger the company runs:\n${table}\n` +
			`a message sent through it lands on whichever one capabilityd was told, and answers "sent" either way`
	);
}

export async function aCompanyPlane(request: PlaneRequest = {}): Promise<ACompanyPlane> {
	const messengerPlatform = request.messengerPlatform ?? 'buzz';
	const runIdentifier = crypto.randomUUID().slice(0, 8);
	const projectURL = environmentValue('SUPABASE_URL');
	const serviceRoleKey = environmentValue('SUPABASE_SECRET_KEY');
	const binaryDirectory = environmentValue('COMPANY_PLANE_BIN');

	// A plane that disagrees with itself is refused before anything is created, so
	// proving it refuses costs no directory to clean up.
	const capabilitydPlatform =
		request.disagreeAbout === 'capabilityd' ? 'mattermost' : messengerPlatform;
	const admindPlatform = request.disagreeAbout === 'admind' ? 'mattermost' : messengerPlatform;
	refuseAPlaneThatDisagreesWithItself({
		capabilityd: capabilitydPlatform,
		admind: admindPlatform,
		connector: messengerPlatform
	});

	const runDirectory = join(repositoryRoot, '.local', 'company-plane', runIdentifier);
	// macOS caps a unix socket path at 104 bytes, and a repository path plus a run
	// identifier gets close, so the sockets live somewhere short.
	const socketDirectory = join(tmpdir(), `ikplane-${runIdentifier}`);
	mkdirSync(join(runDirectory, 'state'), { recursive: true });
	mkdirSync(join(runDirectory, 'secrets'), { recursive: true });
	mkdirSync(socketDirectory, { recursive: true });

	const admin = createClient(projectURL, serviceRoleKey, {
		auth: { autoRefreshToken: false, persistSession: false }
	});

	const connector = aConnectorNobodyRuns();
	const messenger = aMessengerNobodyRuns();
	const model = aModelNobodyPaysFor();
	const started: Bun.Subprocess[] = [];
	let companyID = '';
	let droppableDatabase = '';

	// A run that failed is the one whose logs somebody wants. Removing them on the
	// way out is how a sandbox becomes as hard to diagnose as the thing it replaced.
	let broughtUp = false;

	const stop = async () => {
		for (const child of started.reverse()) {
			child.kill('SIGTERM');
			await Promise.race([child.exited, Bun.sleep(2000)]);
			child.kill('SIGKILL');
		}
		connector.stop();
		messenger.stop();
		model.stop();
		if (companyID) {
			await admin.from('company').delete().eq('id', companyID);
		}
		if (droppableDatabase) {
			await runPostgres(`DROP DATABASE IF EXISTS ${droppableDatabase} WITH (FORCE)`).catch(() => {});
		}
		rmSync(socketDirectory, { recursive: true, force: true });
		const keepAsked = request.keepRunDirectory || process.env.COMPANY_PLANE_KEEP === '1';
		if (broughtUp && !keepAsked) {
			rmSync(runDirectory, { recursive: true, force: true });
		} else if (!broughtUp) {
			console.error(`[plane] the plane did not come up; its logs are in ${runDirectory}`);
		}
	};

	try {
		const company = await provisionCompany(
			admin,
			{
				name: '평면 점검',
				slug: `plane-${runIdentifier}`,
				country: 'KR',
				locale: 'ko',
				timezone: 'Asia/Seoul'
			},
			`sample-${runIdentifier}@example.test`
		);
		companyID = company.companyID;
		const colleagueEmail = `example-${runIdentifier}@example.test`;
		const colleagueID = await addMember(admin, companyID, colleagueEmail);
		await admin.from('member').update({ name: '이샘플' }).eq('id', company.adminMemberID);
		await admin.from('member').update({ name: '박예시' }).eq('id', colleagueID);
		// A member without an account is on the roster and cannot act, which the
		// public API answers as "not active member" — so both of them sign in.
		await arrive(admin, `sample-${runIdentifier}@example.test`);
		await arrive(admin, colleagueEmail);
		const agent = await issueAgentKey(admin, companyID, `plane-${runIdentifier}`);

		const agentKeyPath = join(runDirectory, 'secrets', 'agent-key');
		writeFileSync(agentKeyPath, `${agent.apiKey}\n`, { mode: 0o600 });
		writeFileSync(join(runDirectory, 'secrets', 'buzz-key-seed'), `${crypto.randomUUID()}\n`, {
			mode: 0o600
		});
		// No model is called here — a wiring scenario asks nobody to think — but the
		// processes read the path at startup, so it has to be a file.
		const openRouterKeyPath = join(runDirectory, 'secrets', 'openrouter-key');
		writeFileSync(openRouterKeyPath, 'no-model-is-called-here\n', { mode: 0o600 });

		const admindPort = await aFreePort();
		const blueclawPort = await aFreePort();
		const blueclawURL = `http://127.0.0.1:${blueclawPort}`;
		const admindURL = `http://127.0.0.1:${admindPort}`;
		const capabilitySocketPath = join(socketDirectory, 'capability.sock');
		const blueclawACPSocketPath = join(socketDirectory, 'blueclaw-acp.sock');
		const arrivalsPort = await aFreePort();
		const relayInboundURL = `http://127.0.0.1:${arrivalsPort}/inbound`;
		// The public API only trusts a requester that arrived on this socket, which is
		// the door the relay uses; over TCP the same call is an anonymous 401.
		const requesterSocketPath = join(socketDirectory, 'admind.sock');

		started.push(
			Bun.spawn(
				[
					join(binaryDirectory, 'internkim-capabilityd'),
					...theArgumentsThatStart(
						'internkim-capabilityd',
						{
							'--socket': capabilitySocketPath,
							'--openrouter-key': openRouterKeyPath,
							'--local-inference-mode': 'remote',
							'--blueclaw-url': blueclawURL,
							'--admind-url': admindURL,
							'--chatd-endpoint': connector.url,
							'--chatd-platform': capabilitydPlatform
						},
						{ '--admind-socket': requesterSocketPath }
					)
				],
				{ ...logsTo(join(runDirectory, 'capabilityd.log')), env: theBoxEnvironment() }
			)
		);
		// A unix socket is not a file Bun.file() can answer for, so this asks the
		// filesystem the way the entrypoint's `[ ! -S ]` does.
		await untilReady('capabilityd', async () => existsSync(capabilitySocketPath));

		// blueclaw keeps its own record, so the run gets a database of its own inside
		// the one the local stack already runs. Sharing a schema between two runs is
		// how a harness starts needing a reaper.
		const databaseName = `plane_${runIdentifier}`;
		await runPostgres(`CREATE DATABASE ${databaseName}`);
		droppableDatabase = databaseName;

		// The same renderer the container calls. A sandbox that writes its own
		// runtime document proves nothing about the one a company runs on.
		const runtimeConfigurationPath = join(runDirectory, 'runtime.json');
		const policyPath = join(runDirectory, 'policy.json');
		// blueclaw starts with nobody in it, exactly as host/entrypoint.sh writes it
		// when no policy is mounted. Who works here arrives the way it arrives on a
		// real box: admind reads the company's roster and reconciles it on.
		writeFileSync(
			policyPath,
			'{"people":[],"circles":[],"circleSync":{},"resourceAccess":[],"channels":[],"retention":{}}\n'
		);

		const render = Bun.spawnSync(
			[
				join(repositoryRoot, 'tools', 'render-company-runtime'),
				'--template',
				join(repositoryRoot, 'host', 'runtime.template.json'),
				'--capabilityd',
				join(binaryDirectory, 'internkim-capabilityd'),
				'--out',
				runtimeConfigurationPath,
				'--work',
				runDirectory
			],
			{
				env: {
					...theBoxEnvironment(),
					DATABASE_URL: databaseURLFor(databaseName),
					MESSENGER_PLATFORM: messengerPlatform,
					BLUECLAW_BASE_URL: blueclawURL,
					CAPABILITY_SOCKET_PATH: capabilitySocketPath,
					CHATD_ENDPOINT: connector.url,
					MODEL_ENDPOINT: model.url,
					MODEL_API_KEY_PATH: openRouterKeyPath,
					WORKSPACE_ROOT_PATH: join(runDirectory, 'workspace'),
					MIGRATION_DIRECTORY_PATH: join(repositoryRoot, '.dependency', 'blueclaw', 'migrations'),
					LOG_DIRECTORY_PATH: join(runDirectory, 'logs')
				}
			}
		);
		if (render.exitCode !== 0) {
			throw new Error(`render-company-runtime failed: ${render.stderr.toString()}`);
		}

		started.push(
			Bun.spawn(
				[
					join(binaryDirectory, 'blueclaw'),
					...theArgumentsThatStart(
						'blueclaw',
						{
							'-runtime': runtimeConfigurationPath,
							'-policy': policyPath,
							'-acp-socket': blueclawACPSocketPath
						},
						{ '-inbound': request.inbound ?? 'connectors' }
					)
				],
				{
					...logsTo(join(runDirectory, 'blueclaw.log')),
					env: {
						...theBoxEnvironment(),
						BLUECLAW_BUNDLED_SKILLS_PATH: environmentValue('COMPANY_PLANE_SKILLS')
					}
				}
			)
		);
		await untilReady('blueclaw', async () => {
			const answer = await fetch(`${blueclawURL}/admin/api/health`).catch(() => null);
			return answer?.ok ?? false;
		}, 60);

		started.push(
			Bun.spawn(
				[
					join(binaryDirectory, 'internkim-admind'),
					...theArgumentsThatStart(
						'internkim-admind',
						{
							'-listen': `127.0.0.1:${admindPort}`,
							'-capability-socket': capabilitySocketPath,
							'-chatd-endpoint': connector.url,
							'-chatd-platform': admindPlatform,
							'-blueclaw-url': blueclawURL,
							'-blueclaw-policy': policyPath,
							'-buzz-key-seed-path': join(runDirectory, 'secrets', 'buzz-key-seed'),
							'-site-scaffold': join(environmentValue('COMPANY_PLANE_SKILLS'), 'website', 'assets', 'scaffold', 'app'),
							'-central-plane-app-url': environmentValue('INTERNKIM_APP_URL'),
							'-central-plane-agent-key': agentKeyPath,
							'-central-plane-project-url': projectURL,
							'-central-plane-publishable-key': environmentValue('SUPABASE_PUBLISHABLE_KEY')
						},
						{
							'-listen-socket': requesterSocketPath,
							'-state-dir': join(runDirectory, 'state'),
							'-database': join(runDirectory, 'state', 'internkim.sqlite')
						}
					)
				],
				{ ...logsTo(join(runDirectory, 'admind.log')), env: theBoxEnvironment() }
			)
		);
		await untilReady('admind', async () => {
			const answer = await fetch(`${admindURL}/admin/api/health`).catch(() => null);
			return answer?.ok ?? false;
		}, 60);

		// admind reconciles the company's roster onto blueclaw as it starts, which is
		// why it starts after it. Nobody here is written by hand: they arrive the way
		// they arrive on a real box.
		await untilReady('the company roster on blueclaw', async () => {
			const policy = await fetch(`${blueclawURL}/admin/api/policy`)
				.then((answer) => (answer.ok ? (answer.json() as Promise<{ people?: unknown[] }>) : null))
				.catch(() => null);
			return (policy?.people?.length ?? 0) >= 2;
		}, 60);

		// The relay is the ACP client, and the door a messenger connector hands an
		// inbound message to. It is started last because it opens a session against
		// blueclaw's socket, and admind has to have put the roster on blueclaw first
		// or the requester the session names resolves to nobody.
		started.push(
			Bun.spawn(
				['bun', 'run', join(repositoryRoot, 'host', 'relay', 'relay.ts')],
				{
					...logsTo(join(runDirectory, 'relay.log')),
					env: {
						...theBoxEnvironment(),
						SUPABASE_URL: projectURL,
						SUPABASE_PUBLISHABLE_KEY: environmentValue('SUPABASE_PUBLISHABLE_KEY'),
						INTERNKIM_APP_URL: environmentValue('INTERNKIM_APP_URL'),
						MESSENGER_PLATFORM: messengerPlatform,
						AGENT_API_KEY_PATH: agentKeyPath,
						CHATD_BASE_URL: connector.url,
						ADMIND_SOCKET_PATH: requesterSocketPath,
						ADMIND_BASE_URL: admindURL,
						BLUECLAW_ACP_SOCKET_PATH: blueclawACPSocketPath,
						WORKSPACE_ROOT_PATH: join(runDirectory, 'workspace'),
						ARRIVALS_PORT: String(arrivalsPort)
					}
				}
			)
		);
		// The inbound door answers 405 to anything but a POST, which is the cheapest
		// proof that this relay is listening and not somebody else's.
		await untilReady('the relay', async () => {
			const answer = await fetch(relayInboundURL, { method: 'GET' }).catch(() => null);
			return answer?.status === 405;
		}, 60);

		// A plane is not up when its processes are: it is up when it can say who works
		// here.
		await untilReady('the company directory', async () => {
			const answer = await fetch(`${admindURL}/admin/api/directory/people`).catch(() => null);
			if (!answer?.ok) return false;
			const document = (await answer.json()) as { people?: unknown[] };
			return (document.people?.length ?? 0) >= 2;
		}, 60);

		broughtUp = true;
		return {
			runIdentifier,
			companyID,
			admin,
			people: [
				{ email: `sample-${runIdentifier}@example.test`, name: '이샘플', memberID: company.adminMemberID },
				{ email: colleagueEmail, name: '박예시', memberID: colleagueID }
			],
			agentAPIKey: agent.apiKey,
			admindURL,
			blueclawURL,
			requesterSocketPath,
			capabilitySocketPath,
			blueclawACPSocketPath,
			relayInboundURL,
			connector,
			model,
			runtimeConfigurationPath,
			messenger,
			messengerPlatform,
			stop
		};
	} catch (failure) {
		await stop();
		throw failure;
	}
}
