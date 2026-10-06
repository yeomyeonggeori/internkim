export function attendancePreviewEnabled(): boolean {
 return import.meta.env.DEV && import.meta.env.VITE_ATTENDANCE_PREVIEW === '1'
  && typeof window !== 'undefined' && ['localhost', '127.0.0.1', '[::1]'].includes(window.location.hostname);
}
