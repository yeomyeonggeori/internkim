import {expect,test} from '@playwright/test';
import {signInToAttendance,openMonthlyAttendance} from './attendance-central-test-utils';
import {member1Email} from './central-test-utils';
test.use({locale:'ko-KR'});
for(const width of [320,390,1280]) {
 test(`the own clock and team dashboard remain together at ${width}px`,async({page})=>{
  await page.setViewportSize({width,height:844});
  await signInToAttendance(page);
  await expect(page.getByTestId('attendance-team-dashboard')).toBeVisible();
  const strip=page.getByTestId('attendance-own-strip');
  await expect(strip).toBeVisible();
  const action=strip.getByRole('button',{name:/^(출근|퇴근)$/});
  await expect(action).toHaveCount(1);
  await expect(action).toBeVisible();
  const bounds=await action.boundingBox();expect(bounds).not.toBeNull();expect(bounds!.x).toBeGreaterThanOrEqual(0);expect(bounds!.x+bounds!.width).toBeLessThanOrEqual(width);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 });
}
test('mobile navigation keeps own leave reachable and groups administrator views',async({page})=>{
 await page.setViewportSize({width:390,height:844});await signInToAttendance(page);
 await page.getByRole('button',{name:'내 휴가',exact:true}).click();await expect(page.getByTestId('leave-history-view')).toBeVisible();
 await page.getByRole('button',{name:'관리',exact:true}).click();await page.getByRole('menuitem',{name:/휴가 승인/}).click();await expect(page.getByTestId('leave-approval-view')).toBeVisible();
 await page.getByRole('button',{name:'관리',exact:true}).click();await page.getByRole('menuitem',{name:/휴가 관리/}).click();await expect(page.getByTestId('leave-management-view')).toBeVisible();
});
test('a person month remains scoped and reachable on mobile',async({page})=>{
 await page.setViewportSize({width:390,height:844});await signInToAttendance(page);await openMonthlyAttendance(page);
 const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible();
 await expect(page.getByTestId(`team-status-person-header-${member1Email}`)).toBeVisible();
 const previous=dialog.getByRole('button',{name:'이전 달',exact:true});const next=dialog.getByRole('button',{name:'다음 달',exact:true});
 for(const button of [previous,next]) {const bounds=await button.boundingBox();expect(bounds).not.toBeNull();expect(bounds!.x).toBeGreaterThanOrEqual(0);expect(bounds!.x+bounds!.width).toBeLessThanOrEqual(390);}
});
test('desktop places the own action in content and keeps it once after resizing',async({page})=>{
 await page.setViewportSize({width:1280,height:900});await signInToAttendance(page);
 await expect(page.getByTestId('attendance-sidebar-scroll').getByTestId('attendance-own-strip')).toHaveCount(0);
 await page.setViewportSize({width:390,height:844});await expect(page.getByTestId('attendance-own-strip')).toHaveCount(1);await expect(page.getByTestId('attendance-team-dashboard')).toBeVisible();
 await page.setViewportSize({width:1280,height:900});await expect(page.getByTestId('attendance-own-strip')).toHaveCount(1);
});
