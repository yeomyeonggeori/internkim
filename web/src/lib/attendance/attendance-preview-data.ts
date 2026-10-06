import { defaultLeavePolicy } from './leave-policy-defaults';
import { defaultWorkPolicy } from './work-policy-defaults';
import type { AttendanceTeamPage } from './team-page';
const companyID = 'preview-company';
const companyName = '샘플컴퍼니';
const timeZone = 'Asia/Seoul';
const now = new Date();
const serverTime = now.toISOString();
const today = new Intl.DateTimeFormat('sv-SE', {timeZone}).format(now);
const instant = (minutes: number) => new Date(Date.parse(today + 'T08:00:00+09:00') + minutes * 60000).toISOString();
const teamNames = ['제품개발팀','디자인팀','사업운영팀','고객경험팀','마케팅팀','경영지원팀','리서치팀',companyName];
const noTeams = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('teams') === 'none';
export const previewMembers: AttendanceTeamPage['members'] = Array.from({length: 210}, (_, index) => ({
 memberID: `sample-${index+1}`, name: ['이샘플','박예시','최견본'][index % 3] + String(index+1).padStart(3,'0'),
 email: `sample${String(index+1).padStart(3,'0')}@example.com`,
 teamKey: noTeams || index % 8 === 7 ? 'unassigned' : `team-${index % 8}`,
 status: (['working','working','working','done','away','not_started','working'] as const)[index % 7],
 latestAt: instant(index % 65), location: index % 7 === 5 ? null : ['사무실','재택','외근'][index % 3]
}));
const statusOrder = {working:0,needs_checkout:0,away:1,done:2,not_started:3};
const compareMembers = (a: AttendanceTeamPage['members'][number], b: AttendanceTeamPage['members'][number]) => statusOrder[a.status]-statusOrder[b.status] || a.name.localeCompare(b.name,'ko') || a.memberID.localeCompare(b.memberID);
const totals = (members: AttendanceTeamPage['members']) => ({memberCount: members.length,
 working: members.filter(x=>x.status==='working').length, done: members.filter(x=>x.status==='done').length,
 away: members.filter(x=>x.status==='away').length, notStarted: members.filter(x=>x.status==='not_started').length,
 needsCheckout: 0});
const teams: AttendanceTeamPage['teams'] = (noTeams ? [companyName] : teamNames).map((name,index) => {
 const teamKey = noTeams || index===7 ? 'unassigned' : `team-${index}`;
 const members = previewMembers.filter(x=>x.teamKey===teamKey);
 const actors = members.filter(x=>x.latestAt).map(x=>({memberID:x.memberID,name:x.name,email:x.email,occurredAt:x.latestAt!,location:x.location})).sort((a,b)=>b.occurredAt.localeCompare(a.occurredAt));
 return {teamKey,name,...totals(members),recentClockIns:actors.slice(0,5),recentClockOuts:actors.filter(x=>members.find(m=>m.memberID===x.memberID)?.status==='done').slice(0,5),
 recordedLocations: ['사무실','재택','외근'].map(name=>({name,count:members.filter(x=>x.status==='working'&&x.location===name).length})).filter(x=>x.count),unknownLocationCount:0};
});
const settings = {name:companyName,locale:'ko',timeZone,currencyCode:'KRW',workLocations:[{name:'사무실',color:null},{name:'재택',color:null},{name:'외근',color:null}],leaveDays:15,teamViewVisibleToAll:true,profileImageURL:null};
const isAdmin = typeof window === 'undefined' || new URLSearchParams(window.location.search).get('role') !== 'employee';
const own = previewMembers[0];
const clockFixture = typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('clock') : null;
if (['not_started','not_started_error','no_locations'].includes(clockFixture ?? '')) {own.status = 'not_started';const team=teams.find(team=>team.teamKey===own.teamKey);if(team) Object.assign(team,totals(previewMembers.filter(member=>member.teamKey===team.teamKey)));}
if (clockFixture === 'no_locations') settings.workLocations = [];
const ownEvent = {id:'sample-clock-in',personID:own.memberID,kind:'clock_in' as const,occurredAt:instant(0),location:'사무실'};
const ownEvents = ['not_started','not_started_error','no_locations'].includes(clockFixture ?? '') ? [] : [ownEvent];
const previewState = typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('state') : null;
const dateShift = (days:number) => new Date(Date.parse(today+'T12:00:00Z')+days*86400000).toISOString().slice(0,10);
const previewLeaves = Array.from({length:48}, (_,index) => {
 const member=previewMembers[index%4===0?0:index%6], status=index%3===0?'requested':index%3===1?'approved':'rejected';
 const date=dateShift(status==='requested'?3+index:-index-1);
 return {leaveID:'sample-leave-'+index,personID:member.memberID,person:member.name,kindID:['annual','paid','unpaid'][index%3],kind:['연차','유급 휴가','무급휴가'][index%3],days:1,status,isPaid:index%3!==2,isDeducted:index%3===0,startDate:date,endDate:date,startsAt:date+'T00:00:00+09:00',endsAt:new Date(Date.parse(date+'T12:00:00Z')+86400000).toISOString().slice(0,10)+'T00:00:00+09:00',note: index%2?'가족 일정':'개인 일정'};
});
export function attendancePreviewAnswer(name: string, input: Record<string, unknown>): unknown {
 if (previewState==='error' && ['attendance_leave_policy_get','attendance_work_policy_get','leave_list','leave_balance'].includes(name)) throw new Error('Synthetic preview read failure');
 if(name==='attendance_leave_policy_get') return defaultLeavePolicy();
 if(name==='attendance_work_policy_get') return {timeZone,workMode:'flexible',policy:{version:1,revisions:[{...defaultWorkPolicy(),effectiveDate:'1970-01-01'}]},people:previewMembers.map(m=>({personID:m.memberID,workHours:null,minimumDailyMinutes:480}))};
 if(name==='company_holiday_list') return {count:0,year:null,holidays:[]};
 if((name==='attendance_team_page_get'||name==='attendance_team_dashboard_get')) {
  const filtered=previewMembers.filter(m=>m.teamKey===input.selectedTeamKey && (!input.searchText || (m.name+' '+m.email).toLowerCase().includes(String(input.searchText).toLowerCase())) && (!input.locationFilter || m.location===input.locationFilter)).sort(compareMembers);
  const teamOffset=Number(input.teamOffset||0),teamLimit=Number(input.teamLimit||6),memberOffset=Number(input.memberOffset||0),memberLimit=Number(input.memberLimit||24);
  return {companyID,companyName,companySummary:totals(previewMembers),timeZone,serverTime,authorization:{isAdmin,teamViewVisibleToAll:true},teamOffset,teamLimit,teamTotal:teams.length,teams:teams.slice(teamOffset,teamOffset+teamLimit),selectedTeamKey:input.selectedTeamKey||null,memberOffset,memberLimit,memberTotal:filtered.length,members:filtered.slice(memberOffset,memberOffset+memberLimit)};
 }
 if(name==='attendance_current_get') return {memberID:own.memberID,email:own.email,companyID,timeZone,serverTime,backdatedAfterMinutes:60,workLocations:settings.workLocations,authorization:{isAdmin,teamViewVisibleToAll:true},todayEvents:ownEvents,latestEvent:ownEvents.at(-1) ?? null,activeLeave:null};
 if(name==='attendance_add' && clockFixture !== 'not_started') return (async () => {
  await new Promise(resolve=>setTimeout(resolve,300));
  if(['synthetic-error','not_started_error'].includes(clockFixture ?? '')) throw new Error('Synthetic clock failure; no record changed');
  const kind = input.kind === 'clock_out' ? 'clock_out' : 'clock_in';
  const event = {id:'synthetic-clock-'+(ownEvents.length+1),personID:own.memberID,kind,occurredAt:new Date().toISOString(),location:kind==='clock_in'?String(input.location || '사무실'):null};
  ownEvents.push(event as typeof ownEvent);
  own.status = kind==='clock_out'?'done':'working';if(kind==='clock_in') own.location=event.location;
  const team=teams.find(team=>team.teamKey===own.teamKey);if(team){Object.assign(team,totals(previewMembers.filter(member=>member.teamKey===team.teamKey)));team.recordedLocations=['사무실','재택','외근'].map(name=>({name,count:previewMembers.filter(member=>member.teamKey===team.teamKey && member.status==='working' && member.location===name).length})).filter(value=>value.count);}
  return {status:'added',event};
 })();
 if(name==='company_settings_get') return settings;
 if(name==='person_list' && input.limit) {const people=previewMembers.filter(m=>!input.searchText||(m.name+' '+m.email).toLowerCase().includes(String(input.searchText).toLowerCase())).slice(0,Number(input.limit)).map(m=>({personID:m.memberID,name:m.name,email:m.email}));return {requesterID:own.memberID,count:people.length,people};}
 if(name==='person_list') return {requesterID:own.memberID,count:previewMembers.length,people:previewMembers.map(m=>({personID:m.memberID,name:m.name,email:m.email,isAdmin:isAdmin && m.memberID===own.memberID}))};
 if((name==='attendance_list'||name==='attendance_changes_page_get')) {
  const requested=Array.isArray(input.personHints)?previewMembers.filter(m=>(input.personHints as string[]).includes(m.memberID)):input.scope==='all'?previewMembers:[own];
  if (input.handWrittenOnly) {
   const changes = previewState==='empty'?[]:Array.from({length:62}, (_,index)=>{
    const m=previewMembers[index%16], actor=index%3===0?previewMembers[1]:m, date=dateShift(-Math.floor(index/6));
    return {eventID:'sample-change-'+index,personID:m.memberID,person:m.name,personEmail:m.email,changedByID:actor.memberID,kind:index%2?'clock_out':'clock_in',date,time:index%2?'17:30':'08:30',occurredAt:date+(index%2?'T17:30:00+09:00':'T08:30:00+09:00'),location:m.location,wasCorrected:index%2===0,originalDate:index%2===0?date:null,originalTime:index%2===0?(index%4===0?'08:30':'08:00'):null,previousRecorded:index%4!==2,originalLocation:index%2===0 && index%4!==2?'외근':null,originalOccurredAt:index%2===0?date+'T08:00:00+09:00':null,teamKey:m.teamKey,reason:index%2?'기록 누락':index%4===0?'근무지 정정':'시간 정정',teamName:teamNames[index%8],changedByName:actor.name,changedByEmail:actor.email,changedBySource:index%3===0?'observed':'legacy_subject',changedAt:index%3===0?date+'T19:00:00+09:00':null};
   }).filter(row=>(!input.selectedTeamKey||row.teamKey===input.selectedTeamKey)&&(!input.selectedChangedByID||row.changedByID===input.selectedChangedByID)&&(!input.from||row.date>=String(input.from))&&(!input.to||row.date<=String(input.to))&&(!input.teamSearch||row.teamName.includes(String(input.teamSearch)))&&(!input.changedBySearch||(row.changedByName+' '+row.changedByEmail).includes(String(input.changedBySearch))));
   const pageOffset=Number(input.pageOffset||0),pageLimit=Number(input.pageLimit||24),attendance=changes.slice(pageOffset,pageOffset+pageLimit);
   return {scope:'everyone',personID:null,personName:'',from:input.from,to:input.to,serverTime,backdatedAfterMinutes:60,totalCount:changes.length,pageOffset,pageLimit,count:attendance.length,attendance};
  }
  if (input.to === today && input.from === new Date(Date.parse(today+'T12:00:00Z')-86400000).toISOString().slice(0,10)) {
   const attendance = requested.filter(m => m.status !== 'not_started' && m.status !== 'away').flatMap(m => {
    const base = {personID:m.memberID,person:m.name,date:today,location:m.location,wasCorrected:false,originalDate:null,originalTime:null,originalOccurredAt:null,reason:null};
    return [{...base,eventID:m.memberID+'today-in',kind:'clock_in',time:'08:00',occurredAt:instant(0)}, ...(m.status==='done' ? [{...base,eventID:m.memberID+'today-out',kind:'clock_out',time:'09:00',occurredAt:instant(60)}] : [])];
   });
   return {from:input.from,to:input.to,serverTime,backdatedAfterMinutes:60,count:attendance.length,attendance};
  }
  const attendance=requested.flatMap(m=>Array.from({length:4},(_,day)=>{const date=String(input.to||today).slice(0,7)+'-'+String(day+1).padStart(2,'0');return ['clock_in','clock_out'].map((kind,j)=>({eventID:m.memberID+date+kind,personID:m.memberID,person:m.name,kind,date,time:j?'17:30':'08:30',occurredAt:date+(j?'T17:30:00+09:00':'T08:00:00+09:00'),location:m.location,wasCorrected:false,originalDate:null,originalTime:null,originalOccurredAt:null,reason:null}));}).flat());
  return {from:input.from,to:input.to,serverTime,backdatedAfterMinutes:60,count:attendance.length,attendance};
 }
 if(name==='leave_list') {
  const leave = previewState==='empty'?[]:previewLeaves.filter(row=>(input.scope==='all'||(Array.isArray(input.personHints)?input.personHints.includes(row.personID):row.personID===own.memberID))&&(!input.status||row.status===input.status)&&(!input.from||row.endDate>=String(input.from))&&(!input.to||row.startDate<=String(input.to)));
  return {count:leave.length,leave,registeredKinds:['annual','paid','unpaid']};
 }
 if(name==='leave_balance') {
  const people=input.scope==='all'?previewMembers:input.personHint?previewMembers.filter(m=>m.email===input.personHint||m.memberID===input.personHint):[own];
  return {scope:input.scope||'mine',year:now.getFullYear(),count:people.length,balances:people.map(m=>({personID:m.memberID,personName:m.name,grantedDays:15,remainingDays:12,usedDays:3,tracking:'tracked'}))};
 }
 if(name==='person_picture_list') return {count:0,pictures:[]};
 throw new Error(`Local attendance preview does not perform ${name}`);
}
