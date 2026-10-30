import { execFileSync } from 'node:child_process';
import { availableParallelism } from 'node:os';
import { defineConfig, devices } from '@playwright/test';
import { workerCompanyCount } from './tests/e2e/support/worker-company';

const companyTimeZone = 'Asia/Seoul';
process.env.TZ = companyTimeZone;

type Plane = 'central' | 'fixture' | 'mocked' | 'standalone';

const namedPlanes: Exclude<Plane, 'standalone'>[] = ['central', 'fixture', 'mocked'];
const planes: Plane[] = [...namedPlanes, 'standalone'];

const withoutCentralPlane = { SUPABASE_URL: '', SUPABASE_PUBLISHABLE_KEY: '', SUPABASE_SECRET_KEY: '' };

const planeEnvironments: Record<Plane, Record<string, string>> = {
	central: {
		SUPABASE_URL: process.env.SUPABASE_URL ?? '',
		SUPABASE_PUBLISHABLE_KEY: process.env.SUPABASE_PUBLISHABLE_KEY ?? '',
		SUPABASE_SECRET_KEY: process.env.SUPABASE_SECRET_KEY ?? ''
	},
	fixture: {
		SUPABASE_URL: 'http://127.0.0.1:56801',
		SUPABASE_PUBLISHABLE_KEY: 'nonfunctional-fixture-key',
		SUPABASE_SECRET_KEY: '',
		GATEWAY_URL: 'http://127.0.0.1:56802'
	},
	mocked: {
		...withoutCentralPlane,
		VITE_MOCK_ADMIN: '1',
		VITE_MOCK_CRM: '1',
		VITE_MOCK_FILES: '1',
		VITE_MOCK_MAIL: '1',
		VITE_MOCK_MEMORY: '1',
		VITE_MOCK_TASKS: '1',
		VITE_ATTENDANCE_PREVIEW: '1'
	},
	standalone: withoutCentralPlane
};

const freePortsProgram = `
const servers = Array.from({ length: ${planes.length + 1} }, () => Bun.listen({ hostname: '127.0.0.1', port: 0, socket: { data() {} } }));
console.log(servers.map((server) => server.port).join(','));
for (const server of servers) server.stop(true);
`;

process.env.E2E_SERVER_PORTS ??= execFileSync('bun', ['-e', freePortsProgram], { encoding: 'utf8' }).trim();
const serverPorts = process.env.E2E_SERVER_PORTS.split(',');

function baseURLOf(plane: Plane): string {
	return `http://127.0.0.1:${serverPorts[planes.indexOf(plane)]}`;
}

const exchangeRateStandInPort = serverPorts[planes.length];
const exchangeRateStandInURL = `http://127.0.0.1:${exchangeRateStandInPort}/v2`;

const exchangeRateStandIn = {
	command: `bun tests/support/exchange-rate-stand-in.ts ${exchangeRateStandInPort}`,
	url: `${exchangeRateStandInURL}/currencies`,
	reuseExistingServer: false
};

function specsOn(plane: Exclude<Plane, 'standalone'>): RegExp {
	return new RegExp(`-${plane}\\.spec\\.ts$`);
}

const planesServedFromDevelopment: Plane[] = ['mocked', 'standalone'];

function projectsNamedIn(argumentList: string[]): string[] {
	return argumentList.flatMap((argument, index) => {
		if (argument.startsWith('--project=')) return [argument.slice('--project='.length)];
		if (argument === '--project') return [argumentList[index + 1] ?? ''];
		return [];
	});
}

function selectedPlanes(): Plane[] {
	const named = projectsNamedIn(process.argv);
	const selected = planes.filter((plane) => named.includes(plane));
	return selected.length > 0 ? selected : planes;
}

function serverCommandFor(plane: Plane, port: string, isFirstBuiltPlane: boolean): string {
	const hostAndPort = `--host 127.0.0.1 --port ${port} --strictPort`;
	if (planesServedFromDevelopment.includes(plane)) return `bun run dev ${hostAndPort}`;
	const preview = `bunx vite preview ${hostAndPort}`;
	return isFirstBuiltPlane ? `bun run build && ${preview}` : preview;
}

function webServersFor(selected: Plane[]) {
	const firstBuiltPlane = selected.find((plane) => !planesServedFromDevelopment.includes(plane));
	return selected.map((plane) => {
		const url = baseURLOf(plane);
		return {
			command: serverCommandFor(plane, new URL(url).port, plane === firstBuiltPlane),
			url,
			timeout: plane === firstBuiltPlane ? 600_000 : 120_000,
			reuseExistingServer: false,
			env: { VITE_ADMIND_TARGET: '', FRANKFURTER_URL: exchangeRateStandInURL, ...planeEnvironments[plane] }
		};
	});
}

export default defineConfig({
	testDir: './tests/e2e',
	workers: Math.max(1, Math.min(workerCompanyCount, Math.floor(availableParallelism() * 0.3))),
	timeout: 30_000,
	expect: { timeout: 10_000 },
	use: {
		...devices['Desktop Chrome'],
		timezoneId: companyTimeZone,
		trace: 'retain-on-failure'
	},
	projects: [
		...namedPlanes.map((plane) => ({
			name: plane,
			testMatch: specsOn(plane),
			use: { baseURL: baseURLOf(plane) }
		})),
		{ name: 'standalone', testIgnore: namedPlanes.map(specsOn), use: { baseURL: baseURLOf('standalone') } }
	],
	webServer: [exchangeRateStandIn, ...webServersFor(selectedPlanes())],
	reporter: [['list']]
});
