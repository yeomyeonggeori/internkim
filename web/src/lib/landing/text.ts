export const landingText = {
	ko: {
		title: '김인턴 · 개발자가 없는 회사도 바로 쓰는 AI 업무 공간',
		description: '업무, 일정, 근태, 파일, 고객, 대화를 한곳에 두고, 그 안에서 AI 동료가 같이 일합니다.',
		brand: '김인턴',
		navigation: { docs: '문서', github: 'GitHub', signIn: '로그인', changeLanguage: '언어 바꾸기' },
		hero: {
			headline: '부탁하면 처리하는 AI 동료',
			lead: '김인턴은 개발자가 없는 회사도 바로 쓰는 AI 업무 공간입니다. 업무, 일정, 근태, 파일, 고객, 대화가 한곳에 있고, AI 동료가 그 기록을 보고 직접 일을 처리합니다.',
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
					id: 'clockIn',
					tab: '출근',
					turns: [
						{ from: 'person', text: '오늘 재택이야, 출근 기록해 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '재택 출근 기록을 남겼습니다. 2026-11-09(월) 08:13, 재택 기준으로 저장되었습니다.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'meeting',
					tab: '회의',
					turns: [
						{ from: 'person', text: '다음 주에 박예시님이랑 나 둘 다 되는 한 시간 찾아서 회의 잡아 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '두 분 모두 다음 주에 겹치지 않는 시간을 찾아 회의를 등록했습니다.\n\n- **일시**: 11월 19일(목) 14:00~15:00 (1시간)\n- **참석자**: 이샘플, 박예시\n- **일정명**: 팀 미팅 (이샘플·박예시)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(팀 미팅)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'quote',
					tab: '견적서',
					turns: [
						{ from: 'person', text: '이거 회사 양식 견적서 PDF로 만들어 줘.', file: '견적_항목.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '이샘플님, 결과부터 말씀드립니다. 견적서 PDF(견적서_예시상사.pdf)를 첨부합니다.\n\n현재 파일 상태는 다음과 같습니다.\n\n- 문서번호: Q-2026-0142 / 견적일자 2026-11-08 / 유효기간 30일\n- 공급가액 4,690,000원, 부가세 469,000원, 총 5,159,000원', file: '견적서_예시상사.pdf', steps: ['plan', 'bash(office read 견적_항목.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					tab: '자료 링크',
					turns: [
						{ from: 'person', text: '투자사에서 회사 자료를 요청했어. 투자자 범위로 보기 전용 데이터룸 링크 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '투자자 범위로 다운로드 없이 보기만 가능하고 72시간 후 만료되는 데이터룸 링크를 만들까요?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: '응, 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '보기 전용 데이터룸 링크를 만들었습니다.\n\n- **링크**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **접속 코드**: 622332\n- **범위**: 투자자 서클\n- **권한**: 보기 전용 (다운로드 불가)\n- **유효 기간**: 72시간', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: '데이터룸' } }
					]
				}
			]
		},
		workspace: {
			title: '팀의 일이 모이는 앱',
			body: '업무 보드, 회사 캘린더, 출퇴근과 휴가, 조직도, 메신저, 데이터룸, 고객 관리가 한 앱에 있습니다. 김인턴은 사람과 같은 기록을 보고, 처리한 일도 같은 자리에 남깁니다.',
			channels: 'intern.kim에서, 휴대폰 앱에서, 회사 메신저에서'
		},
		reasons: {
			title: '왜 김인턴인가요',
			items: [
				{ title: '처음부터 설명하지 않아도', body: '일과 자료가 한곳에 있으니 AI가 회사의 맥락을 압니다. 누가 부탁하든 회사가 쌓아 온 것에서 시작합니다.' },
				{ title: '구독을 사람 수만큼 사지 않아도', body: '회사 하나의 모델 키로 일마다 맞는 모델을 골라, 쓴 만큼만 냅니다.' },
				{ title: '일하는 동안 쌓이는 데이터룸', body: '일하며 만든 계약서, 견적서, 보고서가 분류되어 쌓입니다. 투자자에게는 링크 하나로 보냅니다.' }
			]
		},
		trust: {
			title: '회사의 일을 맡길 수 있게',
			items: [
				{ title: '시킨 사람의 권한으로만', body: '부탁한 사람이 볼 수 있는 것만 보고, 할 수 있는 것만 합니다. 남긴 기록에는 시킨 사람의 이름이 붙습니다.' },
				{ title: '중요한 일은 먼저 묻기', body: '되돌리기 어려운 일은 하기 전에 부탁한 사람에게 확인을 받습니다.' },
				{ title: '우리가 사라져도', body: '김인턴을 이루는 소프트웨어는 모두 공개되어 있습니다. AI 동료는 회사가 가진 컴퓨터에서 돌아가고, 앱과 데이터베이스까지 직접 운영할 수 있습니다.' }
			]
		},
		hosting: {
			title: 'AI 동료가 일할 컴퓨터',
			items: [
				{ label: '가장 쉽게', title: '김미니를 꽂고 시작', body: '김인턴이 설치된 채로 도착하는 작은 컴퓨터입니다. 전원과 인터넷을 연결하고 화면의 코드를 입력하면 됩니다.' },
				{ label: '가진 장비로', title: '회사 컴퓨터에 설치', body: '리눅스 서버나 Mac처럼 늘 켜 둘 컴퓨터에 명령 두 줄로 설치합니다.' }
			],
			selfHosting: '앱과 데이터베이스까지 직접 운영하려면',
			guide: '셀프호스팅 안내'
		},
		closing: { title: '오늘 회사를 만들고 동료를 초대하세요', quickstart: '빠른 시작 읽기' },
		footer: { madeBy: '여명거리가 만듭니다', license: 'Apache-2.0' }
	},
	en: {
		title: 'Intern Kim · An AI workspace a company without developers can use from day one',
		description: 'Tasks, the calendar, attendance, files, customers and conversations live in one place, and an AI coworker works inside it.',
		brand: 'Intern Kim',
		navigation: { docs: 'Docs', github: 'GitHub', signIn: 'Sign in', changeLanguage: 'Change language' },
		hero: {
			headline: 'An AI coworker that does what you ask',
			lead: 'Intern Kim is an AI workspace a company without developers can use from day one. Tasks, the calendar, attendance, files, customers and conversations live in one place, and an AI coworker reads those records and does the work itself.',
			start: 'Create a company',
			signIn: 'Sign in',
			openSource: 'Every piece of it is open source'
		},
		demo: {
			label: 'A sample conversation with Intern Kim',
			conversation: 'Intern Kim',
			working: 'Intern Kim is working on it',
			scenarios: [
				{
					id: 'clockIn',
					tab: 'Clock in',
					turns: [
						{ from: 'person', text: 'Clock me in — I\'m working from home today.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'You\'re clocked in as of 08:13 KST at Remote.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'meeting',
					tab: 'Meeting',
					turns: [
						{ from: 'person', text: 'Find an hour next week when Park Example and I are both free, and book it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Done. I found an hour next week when you\'re both free and booked it:\n\n- **When**: Thu, Nov 19, 14:00–15:00 (1 hour)\n- **Attendees**: Lee Sample, Park Example\n- **Title**: Team meeting (Lee · Park)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(Team meeting)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'quote',
					tab: 'Quote',
					turns: [
						{ from: 'person', text: 'Turn this into a quote PDF on our company letterhead.', file: 'quote-items.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The quote is attached and filed in the data room. Details:\n\n- Number: Q-2026-0142 · dated 2026-11-08 · valid for 30 days\n- Supply 4,690,000 KRW, VAT 469,000 KRW, total 5,159,000 KRW', file: 'quote-example-co.pdf', steps: ['plan', 'bash(office read quote-items.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					tab: 'Data room link',
					turns: [
						{ from: 'person', text: 'An investor asked for our company materials. Make a view-only data room link for the investor circle.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Create a view-only data room link for the investor circle that expires after 72 hours and does not allow downloads?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: 'Yes, create it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The link is ready.\n\n- **Link**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **Access code**: 622332\n- **Access**: read-only through the Investor circle. Downloading originals is disabled.\n- **Valid for 72 hours**: recipients enter the code and accept the confidentiality notice without an account.\n\nShare the link and code with them by your preferred channel.', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: 'Dataroom' } }
					]
				}
			]
		},
		workspace: {
			title: "An app where the team's work lives",
			body: 'A task board, the company calendar, attendance and leave, the org chart, a messenger, a data room and customer records in one app. Intern Kim reads the same records people do and leaves its work in the same places.',
			channels: 'At intern.kim, on your phone, and in the company messenger'
		},
		reasons: {
			title: 'Why Intern Kim',
			items: [
				{ title: 'No starting from scratch', body: "With the work and its files in one place, the AI knows the company's context. Whoever asks starts from what the company has built up." },
				{ title: 'No seat for every employee', body: 'One company key picks the right model for each job, and you pay for what is used.' },
				{ title: 'A data room that fills as you work', body: 'The contracts, quotes and reports made along the way are filed as you go. An investor gets one link.' }
			]
		},
		trust: {
			title: 'Work you can hand over',
			items: [
				{ title: 'Only what the asker may do', body: 'It sees what the person who asked can see and does what they can do. What it leaves behind carries their name.' },
				{ title: 'It asks before the big things', body: 'Before anything hard to undo, it checks with the person who asked.' },
				{ title: 'Even if we disappear', body: 'Every piece of Intern Kim is open source. The AI coworker runs on a computer your company owns, and you can run the app and the database yourself too.' }
			]
		},
		hosting: {
			title: 'A computer for your AI coworker',
			items: [
				{ label: 'Easiest', title: 'Plug in a Kim mini', body: 'A small computer that arrives with Intern Kim installed. Connect power and the internet, then enter the code it shows.' },
				{ label: 'Your own hardware', title: 'Install on a company computer', body: 'Two commands on a Linux server or Mac that stays on.' }
			],
			selfHosting: 'To run the app and the database yourself too',
			guide: 'Self-hosting guide'
		},
		closing: { title: 'Create your company today and invite your colleagues', quickstart: 'Read the quickstart' },
		footer: { madeBy: 'Made by Yeomyeonggeori', license: 'Apache-2.0' }
	}
} as const;
