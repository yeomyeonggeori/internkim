import type { Plugin } from 'vite';
import {
	createDevAdminMockResponse,
	createDevAdminMockState,
	readRequestBody,
	shouldHandleDevAdminMockRequest,
	writeJSON,
	type DevAdminMockState,
	type DevMockRequest,
	type DevMockResponse
} from './dev-admin-mock';
import type { WorkspaceEntry, WorkspaceRoot } from './src/routes/files/files-api';

type DevFilesMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

const personalPath = '/workspace/private/people/dev-person-admin';
const circlePath = '/workspace/circles/design-team';
const publicPath = '/workspace/shared';

const roots: WorkspaceRoot[] = [
	{ id: 'personal', label: '개인', agentPath: personalPath, kind: 'personal' },
	{ id: 'circle-design', label: '디자인팀', agentPath: circlePath, kind: 'circle' },
	{ id: 'public', label: '공개', agentPath: publicPath, kind: 'public' }
];

function directory(name: string, parent: string, modifiedAt: string): WorkspaceEntry {
	return { name, agentPath: `${parent}/${name}`, isDirectory: true, size: 4096, modifiedAt };
}

function file(name: string, parent: string, size: number, modifiedAt: string): WorkspaceEntry {
	return { name, agentPath: `${parent}/${name}`, isDirectory: false, size, modifiedAt };
}

const listings: Record<string, WorkspaceEntry[]> = {
	[personalPath]: [
		directory('문서', personalPath, '2026-06-21T09:12:00+09:00'),
		directory('이미지', personalPath, '2026-06-18T14:03:00+09:00'),
		directory('프로젝트', personalPath, '2026-06-20T18:45:00+09:00'),
		directory('임시', personalPath, '2026-06-15T10:20:00+09:00'),
		file('주간-회고.md', personalPath, 4213, '2026-06-22T08:30:00+09:00'),
		file('할일.txt', personalPath, 612, '2026-06-22T07:05:00+09:00')
	],
	[`${personalPath}/문서`]: [
		file('2026-상반기-보고서.md', `${personalPath}/문서`, 18244, '2026-06-21T09:12:00+09:00'),
		file('회의록-0620.md', `${personalPath}/문서`, 6120, '2026-06-20T17:40:00+09:00'),
		file('예산-계획.csv', `${personalPath}/문서`, 2048, '2026-06-19T11:20:00+09:00')
	],
	[`${personalPath}/이미지`]: [
		file('로고.svg', `${personalPath}/이미지`, 3120, '2026-06-18T14:03:00+09:00')
	],
	[`${personalPath}/프로젝트`]: [
		file('README.md', `${personalPath}/프로젝트`, 1840, '2026-06-20T18:45:00+09:00'),
		file('main.go', `${personalPath}/프로젝트`, 5312, '2026-06-20T18:30:00+09:00'),
		file('메모.txt', `${personalPath}/프로젝트`, 920, '2026-06-20T18:10:00+09:00')
	],
	[circlePath]: [
		directory('브랜드-가이드', circlePath, '2026-06-17T10:00:00+09:00'),
		file('스프린트-보드.md', circlePath, 7420, '2026-06-21T19:30:00+09:00')
	],
	[`${circlePath}/브랜드-가이드`]: [
		file('컬러-팔레트.md', `${circlePath}/브랜드-가이드`, 2210, '2026-06-17T10:00:00+09:00')
	],
	[publicPath]: [
		file('공지사항.md', publicPath, 1530, '2026-06-22T09:00:00+09:00')
	]
};

const previewContents: Record<string, string> = {
	[`${personalPath}/주간-회고.md`]: `# 주간 회고 (6월 4주차)

## 잘된 점
- 파일 UI 리디자인 착수
- 워크스페이스 명칭 정리

## 개선할 점
- 미리보기 로딩 속도

## 다음 주 할 일
- [ ] 배포 후 디바이스 검증
- [ ] 사용자 피드백 수집
`,
	[`${personalPath}/할일.txt`]: `- 보고서 초안 마무리
- 디자인 리뷰 요청
- 회의록 정리
`,
	[`${personalPath}/문서/예산-계획.csv`]: `항목,1분기,2분기,합계
인건비,1200,1350,2550
장비,300,180,480
운영비,150,200,350
교육,80,120,200`,
	[`${personalPath}/문서/2026-상반기-보고서.md`]: `# 2026 상반기 보고서

상반기 주요 성과와 지표를 정리한 문서입니다.

## 핵심 지표
- 활성 사용자 증가
- 응답 시간 개선
`,
	[`${personalPath}/프로젝트/main.go`]: `package main

import "fmt"

func main() {
	fmt.Println("hello, workspace")
}
`,
	[`${personalPath}/프로젝트/README.md`]: `# 프로젝트

내부 도구 프로토타입입니다.
`
};

export function devFilesMockPlugin(options: DevFilesMockPluginOptions): Plugin {
	const state = createDevAdminMockState(options.userEmail);

	return {
		name: 'internkim-dev-files-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				const pathname = requestURL.pathname;

				if (method === 'GET' && pathname === '/files/api/download') {
					const path = requestURL.searchParams.get('path') ?? '';
					response.statusCode = 200;
					response.setHeader('Content-Type', 'text/plain; charset=utf-8');
					response.end(previewContents[path] ?? '미리보기 내용이 없습니다.');
					return;
				}

				if (!shouldHandleDevFilesMockRequest(method, pathname)) {
					next();
					return;
				}

				readRequestBody(request, (body) => {
					const mockResponse = createDevFilesMockResponse(state, {
						method,
						pathname,
						searchParams: requestURL.searchParams,
						body
					});
					if (!mockResponse) {
						next();
						return;
					}
					writeJSON(response, mockResponse.status, mockResponse.body);
				});
			});
		}
	};
}

function withEntryCount(entry: WorkspaceEntry): WorkspaceEntry {
	if (!entry.isDirectory) return entry;
	return { ...entry, entryCount: listings[entry.agentPath]?.length ?? 0 };
}

function createDevFilesMockResponse(
	state: DevAdminMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/files/api/roots') {
		return { status: 200, body: { roots } };
	}
	if (request.method === 'GET' && request.pathname === '/files/api/list') {
		const path = request.searchParams.get('path') ?? '';
		return { status: 200, body: { entries: (listings[path] ?? []).map(withEntryCount) } };
	}
	if (request.method === 'POST' && request.pathname === '/files/api/upload') {
		return { status: 200, body: { uploaded: [] } };
	}
	return createDevAdminMockResponse(state, request);
}

function shouldHandleDevFilesMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/files/api/roots') return true;
	if (method === 'GET' && pathname === '/files/api/list') return true;
	if (method === 'POST' && pathname === '/files/api/upload') return true;
	return shouldHandleDevAdminMockRequest(method, pathname);
}
