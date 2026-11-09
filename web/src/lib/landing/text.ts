export const landingText = {
	ko: {
		title: '김인턴 · AI 동료와 쓰는 작은 회사의 백오피스',
		description: '근태, 업무, 일정, 고객, 문서를 한 앱에서 관리합니다. 메신저로 부탁하면 김인턴이 회사 기록을 보고 처리합니다.',
		brand: '김인턴',
		navigation: { docs: '문서', github: 'GitHub', signIn: '로그인', changeLanguage: '언어 바꾸기' },
		hero: {
			headline: 'AI 동료와 쓰는 백오피스',
			lead: '근태, 업무, 일정, 고객, 문서를 한 앱에서 관리합니다. 메신저로 일을 부탁하면 김인턴이 회사 기록을 보고 처리합니다.',
			start: '회사 만들기',
			signIn: '로그인',
			openSource: '모든 소프트웨어가 오픈 소스입니다'
		},
		demo: {
			label: '김인턴과 나눈 대화 예시',
			conversation: '김인턴',
			working: '김인턴이 처리하는 중',
			scenarios: [
				{
					id: 'attendance',
					turns: [
						{ from: 'person', text: '오늘 재택이야, 출근 기록해 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '재택 출근 기록을 남겼습니다. 2026-11-09(월) 08:13, 재택 기준으로 저장되었습니다.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'calendar',
					turns: [
						{ from: 'person', text: '다음 주에 박예시님이랑 나 둘 다 되는 한 시간 찾아서 회의 잡아 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '두 분 모두 다음 주에 겹치지 않는 시간을 찾아 회의를 등록했습니다.\n\n- **일시**: 11월 19일(목) 14:00~15:00 (1시간)\n- **참석자**: 이샘플, 박예시\n- **일정명**: 팀 미팅 (이샘플·박예시)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(팀 미팅)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'files',
					turns: [
						{ from: 'person', text: '이거 회사 양식 견적서 PDF로 만들어 줘.', file: '견적_항목.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '이샘플님, 결과부터 말씀드립니다. 견적서 PDF(견적서_예시상사.pdf)를 첨부합니다.\n\n현재 파일 상태는 다음과 같습니다.\n\n- 문서번호: Q-2026-0142 / 견적일자 2026-11-08 / 유효기간 30일\n- 공급가액 4,690,000원, 부가세 469,000원, 총 5,159,000원', file: '견적서_예시상사.pdf', steps: ['plan', 'bash(office read 견적_항목.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					turns: [
						{ from: 'person', text: '투자사에서 회사 자료를 요청했어. 투자자 범위로 보기 전용 데이터룸 링크 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '투자자 범위로 다운로드 없이 보기만 가능하고 72시간 후 만료되는 데이터룸 링크를 만들까요?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: '응, 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '보기 전용 데이터룸 링크를 만들었습니다.\n\n- **링크**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **접속 코드**: 622332\n- **범위**: 투자자 서클\n- **권한**: 보기 전용 (다운로드 불가)\n- **유효 기간**: 72시간', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: '데이터룸' } }
					]
				}
			]
		},
		problems: {
			title: '회사에서 AI를 쓰려면',
			items: [
				{ problem: '회사 설명은 매번 처음부터', answer: '회사 기록을 보고 일합니다', body: '업무, 일정, 고객, 문서가 한 앱에 있습니다. 김인턴은 부탁한 사람이 볼 수 있는 기록을 읽고 일합니다.' },
				{ problem: '도구마다 따로 내는 AI 구독', answer: '좌석 구독에 AI 크레딧이 들어 있습니다', body: '일마다 맞는 모델을 골라 씁니다. 크레딧이 모자라면 더 사거나, 회사 OpenRouter 키를 연결해 모델 비용을 직접 낼 수 있습니다.' },
				{ problem: '파일은 여기저기', answer: '문서는 데이터룸에 모입니다', body: '업무 중에 만든 계약서, 견적서, 보고서가 분류되어 저장됩니다. 투자자에게는 데이터룸 링크를 보냅니다.' }
			]
		},
		stories: {
			attendance: { title: '메신저로 출근을 기록합니다', body: '재택 출근을 기록해 달라고 하면 김인턴이 근태에 남깁니다. 오늘 누가 출근했는지 물으면 팀이 보는 그 기록을 보고 답합니다.' },
			work: { title: '고객 등록부터 회의 예약까지', body: '고객 등록, 회의 예약, 업무 추가를 메신저로 부탁하세요. 김인턴이 한 일은 팀이 쓰는 CRM, 일정, 업무 보드에 그대로 남습니다.' },
			record: { title: '투자자에게 데이터룸 링크를 보냅니다', body: '출근 기록, 고객 정보, 회의 기록, 김인턴이 만든 문서가 데이터룸에 모입니다. 투자자가 자료를 요청하면 링크를 보내세요.' }
		},
		trust: {
			title: '권한과 운영 방식',
			items: [
				{ title: '부탁한 사람의 권한으로만', body: '김인턴은 부탁한 사람이 볼 수 있는 기록만 읽고, 할 수 있는 일만 합니다. 남긴 기록에는 그 사람의 이름이 붙습니다.' },
				{ title: '되돌리기 어려운 일은 먼저 확인합니다', body: '데이터룸 링크 만들기처럼 되돌리기 어려운 일은 부탁한 사람에게 먼저 묻습니다.' },
				{ title: '회사 컴퓨터에서 실행합니다', body: '모든 소프트웨어가 Apache-2.0 오픈 소스이고, AI 동료는 회사가 가진 컴퓨터에서 돌아갑니다.' }
			]
		},
		plans: {
			title: '요금과 설치',
			lead: '구성원 수만큼 구독하고, AI 동료가 일할 컴퓨터를 고르세요.',
			pay: {
				title: '구독과 AI 크레딧',
				items: [
					{ title: '좌석 구독', body: '구성원 수만큼 구독합니다. 구독에 AI 크레딧이 들어 있습니다.' },
					{ title: '크레딧 추가', body: '크레딧이 모자라면 더 살 수 있습니다.' },
					{ title: '회사 OpenRouter 키', body: 'OpenRouter 키를 연결하면 모델 비용을 직접 냅니다.' }
				]
			},
			run: {
				title: 'AI 동료가 일할 컴퓨터',
				items: [
					{ title: '김미니', body: '김인턴이 설치된 채로 도착하는 작은 컴퓨터입니다. 전원과 인터넷만 연결하면 됩니다.' },
					{ title: '회사 컴퓨터', body: '늘 켜 둘 리눅스 서버나 Mac에 명령 두 줄로 설치합니다.' },
					{ title: '직접 운영', body: '앱과 데이터베이스까지 회사가 직접 운영합니다.' }
				],
				guide: '셀프호스팅 안내'
			}
		},
		closing: { title: '회사를 만들고 동료를 초대하세요', quickstart: '빠른 시작 읽기' },
		footer: { madeBy: '여명거리가 만듭니다', license: 'Apache-2.0' }
	},
	en: {
		title: 'Intern Kim · A back office with an AI coworker',
		description: 'Attendance, tasks, calendar, customers and documents in one app. Ask in the messenger, and Intern Kim reads your company records and does the work.',
		brand: 'Intern Kim',
		navigation: { docs: 'Docs', github: 'GitHub', signIn: 'Sign in', changeLanguage: 'Change language' },
		hero: {
			headline: 'A back office with an AI coworker',
			lead: 'Attendance, tasks, calendar, customers and documents in one app. Ask in the messenger, and Intern Kim reads your company records and does the work.',
			start: 'Create a company',
			signIn: 'Sign in',
			openSource: 'Every piece is open source'
		},
		demo: {
			label: 'A sample conversation with Intern Kim',
			conversation: 'Intern Kim',
			working: 'Intern Kim is working on it',
			scenarios: [
				{
					id: 'attendance',
					turns: [
						{ from: 'person', text: 'Clock me in — I\'m working from home today.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'You\'re clocked in as of 08:13 KST at Remote.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'calendar',
					turns: [
						{ from: 'person', text: 'Find an hour next week when Park Example and I are both free, and book it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Done. I found an hour next week when you\'re both free and booked it:\n\n- **When**: Thu, Nov 19, 14:00–15:00 (1 hour)\n- **Attendees**: Lee Sample, Park Example\n- **Title**: Team meeting (Lee · Park)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(Team meeting)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'files',
					turns: [
						{ from: 'person', text: 'Turn this into a quote PDF on our company letterhead.', file: 'quote-items.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The quote is attached and filed in the data room. Details:\n\n- Number: Q-2026-0142 · dated 2026-11-08 · valid for 30 days\n- Supply 4,690,000 KRW, VAT 469,000 KRW, total 5,159,000 KRW', file: 'quote-example-co.pdf', steps: ['plan', 'bash(office read quote-items.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					turns: [
						{ from: 'person', text: 'An investor asked for our company materials. Make a view-only data room link for the investor circle.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Create a view-only data room link for the investor circle that expires after 72 hours and does not allow downloads?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: 'Yes, create it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The link is ready.\n\n- **Link**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **Access code**: 622332\n- **Access**: read-only through the Investor circle. Downloading originals is disabled.\n- **Valid for 72 hours**: recipients enter the code and accept the confidentiality notice without an account.\n\nShare the link and code with them by your preferred channel.', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: 'Dataroom' } }
					]
				}
			]
		},
		problems: {
			title: 'Using AI at a small company',
			items: [
				{ problem: 'Explaining the company every time', answer: 'It reads your company records', body: 'Tasks, calendar, customers and documents are all in the app. Intern Kim reads the records the person asking is allowed to see.' },
				{ problem: 'An AI subscription for every tool', answer: 'AI credits come with each seat', body: 'Intern Kim picks a model for each job. Buy more credits when you run low, or connect your OpenRouter key and pay model costs directly.' },
				{ problem: 'Files scattered everywhere', answer: 'Documents land in the data room', body: 'Contracts, quotes and reports made at work are filed by category. When an investor asks, send a data room link.' }
			]
		},
		stories: {
			attendance: { title: 'Clock in from the messenger', body: "Ask Intern Kim to clock you in from home and it records your attendance. Ask who's in today and it answers from the records your team sees." },
			work: { title: 'From new customer to booked meeting', body: 'Ask Intern Kim to add a customer, book a meeting or create a task. The result shows up in the CRM, calendar and task board your team already uses.' },
			record: { title: 'Send investors a data room link', body: 'Attendance, customers, meetings and the documents Intern Kim makes are collected in the data room. When an investor asks for materials, send a link.' }
		},
		trust: {
			title: 'Permissions and hosting',
			items: [
				{ title: 'Only with your permissions', body: 'Intern Kim reads only the records the person asking can see, and does only what they can do. What it records carries their name.' },
				{ title: 'Asks before anything hard to undo', body: 'Before something hard to undo, like creating a data room link, it asks the person who made the request.' },
				{ title: "Runs on your company's computer", body: 'All software is open source under Apache-2.0, and the AI coworker runs on a computer your company owns.' }
			]
		},
		plans: {
			title: 'Pricing and setup',
			lead: 'Subscribe per member and pick the computer your AI coworker runs on.',
			pay: {
				title: 'Seats and AI credits',
				items: [
					{ title: 'Per seat', body: 'Pay per member. AI credits are included.' },
					{ title: 'More credits', body: 'Buy more when you run low.' },
					{ title: 'Your OpenRouter key', body: 'Connect it and pay model costs directly.' }
				]
			},
			run: {
				title: 'A computer for your AI coworker',
				items: [
					{ title: 'Kim mini', body: 'A small computer that arrives with Intern Kim installed. Connect power and internet.' },
					{ title: 'Your computer', body: 'Install with two commands on a Linux server or Mac that stays on.' },
					{ title: 'Self-hosting', body: 'Run the app and database yourself.' }
				],
				guide: 'Self-hosting guide'
			}
		},
		closing: { title: 'Create a company and invite your team', quickstart: 'Read the quickstart' },
		footer: { madeBy: 'Made by Yeomyeonggeori', license: 'Apache-2.0' }
	}
} as const;
