import type { UserRecord } from './src/routes/admin/admin-types';
import type { DevAdminMockUserRole } from './dev-admin-mock';

type DevAdminOrgchartPerson = {
	userID: string;
	handle: string;
	name: string;
	email: string;
	hireDate: string;
	jobTitle: string;
	groupID?: string;
	supervisorID?: string;
};

const devAdminOrgchartPeople: DevAdminOrgchartPerson[] = [
	{ userID: 'dev-user-ceo', handle: 'ceo', name: '김도형', email: '', hireDate: '2026-01-03', jobTitle: '대표이사', groupID: 'group-leadership' },
	{ userID: 'dev-user-sujin', handle: 'sujin', name: '이수진', email: 'sujin@example.com', hireDate: '2026-02-01', jobTitle: '운영 리드', groupID: 'group-operations', supervisorID: 'dev-user-ceo' },
	{ userID: 'dev-user-junho', handle: 'junho', name: '이정훈', email: 'junho@example.com', hireDate: '2026-02-03', jobTitle: '제품팀 리드', groupID: 'group-product', supervisorID: 'dev-user-ceo' },
	{ userID: 'dev-user-jieun', handle: 'jieun', name: '박지은', email: 'jieun@example.com', hireDate: '2026-02-05', jobTitle: '디자인 리드', groupID: 'group-design', supervisorID: 'dev-user-ceo' },
	{ userID: 'dev-user-woojin', handle: 'woojin', name: '장우진', email: 'woojin@example.com', hireDate: '2026-02-07', jobTitle: '현장지원팀 리드', groupID: 'group-field', supervisorID: 'dev-user-ceo' },
	{ userID: 'dev-user-dabin', handle: 'dabin', name: '김다빈', email: 'dabin@example.com', hireDate: '2026-03-11', jobTitle: '프론트엔드 개발자', groupID: 'group-product', supervisorID: 'dev-user-junho' },
	{ userID: 'dev-user-yuna', handle: 'yuna', name: '정유나', email: 'yuna@example.com', hireDate: '2026-03-18', jobTitle: 'UX 디자이너', groupID: 'group-design', supervisorID: 'dev-user-jieun' },
	{ userID: 'dev-user-minsu', handle: 'minsu', name: '박민수', email: 'minsu@example.com', hireDate: '2026-03-20', jobTitle: '운영 매니저', groupID: 'group-operations', supervisorID: 'dev-user-sujin' },
	{ userID: 'dev-user-seoyeon', handle: 'seoyeon', name: '서서연', email: 'seoyeon@example.com', hireDate: '2026-03-24', jobTitle: '현장 코디네이터', groupID: 'group-field', supervisorID: 'dev-user-woojin' },
	{ userID: 'dev-user-nam', handle: 'nam', name: '남지훈', email: 'nam@example.com', hireDate: '2026-04-02', jobTitle: '사업 개발', supervisorID: 'dev-user-ceo' },
	{ userID: 'dev-user-hyejin', handle: 'hyejin', name: '최혜진', email: 'hyejin@example.com', hireDate: '2026-03-22', jobTitle: '운영 담당', groupID: 'group-operations', supervisorID: 'dev-user-sujin' },
	{ userID: 'dev-user-eunseo', handle: 'eunseo', name: '윤은서', email: 'eunseo@example.com', hireDate: '2026-04-04', jobTitle: '운영 담당', groupID: 'group-operations', supervisorID: 'dev-user-minsu' },
	{ userID: 'dev-user-hanseul', handle: 'hanseul', name: '정한슬', email: 'hanseul@example.com', hireDate: '2026-04-09', jobTitle: '고객 운영', groupID: 'group-operations', supervisorID: 'dev-user-minsu' },
	{ userID: 'dev-user-minjae', handle: 'minjae', name: '강민재', email: 'minjae@example.com', hireDate: '2026-03-16', jobTitle: '백엔드 개발자', groupID: 'group-product', supervisorID: 'dev-user-junho' },
	{ userID: 'dev-user-yewon', handle: 'yewon', name: '오예원', email: 'yewon@example.com', hireDate: '2026-03-25', jobTitle: '프로덕트 매니저', groupID: 'group-product', supervisorID: 'dev-user-junho' },
	{ userID: 'dev-user-taehyun', handle: 'taehyun', name: '신태현', email: 'taehyun@example.com', hireDate: '2026-04-08', jobTitle: 'QA 엔지니어', groupID: 'group-product', supervisorID: 'dev-user-dabin' },
	{ userID: 'dev-user-seojun', handle: 'seojun', name: '조서준', email: 'seojun@example.com', hireDate: '2026-03-27', jobTitle: '그래픽 디자이너', groupID: 'group-design', supervisorID: 'dev-user-yuna' },
	{ userID: 'dev-user-arin', handle: 'arin', name: '배아린', email: 'arin@example.com', hireDate: '2026-04-01', jobTitle: '브랜드 디자이너', groupID: 'group-design', supervisorID: 'dev-user-jieun' },
	{ userID: 'dev-user-sungmin', handle: 'sungmin', name: '송성민', email: 'sungmin@example.com', hireDate: '2026-03-28', jobTitle: '필드 엔지니어', groupID: 'group-field', supervisorID: 'dev-user-woojin' },
	{ userID: 'dev-user-soyoung', handle: 'soyoung', name: '한소영', email: 'soyoung@example.com', hireDate: '2026-04-03', jobTitle: '기술 지원 담당', groupID: 'group-field', supervisorID: 'dev-user-seoyeon' }
];

export function createDevAdminOrgchartUsers(userEmail: string, userRole: DevAdminMockUserRole = 'admin'): UserRecord[] {
	return devAdminOrgchartPeople.map((person) => userRecord(person, userEmail, userRole));
}

function userRecord(person: DevAdminOrgchartPerson, userEmail: string, userRole: DevAdminMockUserRole): UserRecord {
	const email = person.userID === 'dev-user-ceo' ? userEmail : person.email;
	const role = person.userID === 'dev-user-ceo' ? userRole : 'member';
	const groupIDs = person.groupID ? [person.groupID] : [];

	return {
		userID: person.userID,
		handle: person.handle,
		name: person.name,
		email,
		hireDate: person.hireDate,
		role,
		status: 'active',
		jobTitle: person.jobTitle,
		group: person.groupID,
		primaryGroupID: person.groupID,
		groupIDs,
		supervisorID: person.supervisorID
	};
}
