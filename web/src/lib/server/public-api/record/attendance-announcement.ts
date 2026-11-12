export type AttendanceAnnouncement = { what: 'clock'; colleagues: boolean } | { what: 'leave' };

export type AttendanceAnnouncer = (announcement: AttendanceAnnouncement) => Promise<void>;
