/**
 * internkim-bridge — Google Apps Script web app the user deploys once
 * from their own Google account. It exposes a minimal webhook that the
 * on-device agent POSTs to in order to create Slides / Docs / Sheets /
 * Calendar events / Gmail messages *as the user*. Apps Script runs under
 * the deploying user's identity, so these calls never hit Google's
 * "unverified app" block that sinks direct third-party OAuth flows.
 *
 * Every action that creates a Drive file accepts an optional `share_to`
 * parameter (the internkim service-account email). When provided, the
 * file is shared with that email as a writer so subsequent edits from
 * the board (via `gws-bot`) land in the file's revision history as
 * "Intern Kim" rather than the user.
 *
 * Deploy:
 *   1. Open https://script.google.com/home/start and create a new project.
 *   2. Paste this file's contents into the editor.
 *   3. Click Deploy → New deployment → Web app.
 *        - Execute as: Me
 *        - Who has access: Only myself
 *   4. Authorize the prompted scopes (Drive, Slides, Docs, Sheets,
 *      Calendar, Gmail). Google's own consent screen is never blocked.
 *   5. Copy the Web App URL that Apps Script returns and paste it back
 *      into internkim setup when prompted.
 */

function doPost(event) {
  var parameters = event.parameter || {};
  var action = parameters.action;
  try {
    switch (action) {
      case 'slides.create':
        return reply_(createSlides_(parameters));
      case 'docs.create':
        return reply_(createDoc_(parameters));
      case 'sheets.create':
        return reply_(createSheet_(parameters));
      case 'calendar.event':
        return reply_(createCalendarEvent_(parameters));
      case 'gmail.send':
        return reply_(sendGmail_(parameters));
      default:
        return reply_({error: 'unknown action: ' + action});
    }
  } catch (error) {
    return reply_({error: String(error && error.message ? error.message : error)});
  }
}

function doGet(event) {
  return reply_({ok: true, message: 'internkim-bridge alive'});
}

function reply_(payload) {
  return ContentService
    .createTextOutput(JSON.stringify(payload))
    .setMimeType(ContentService.MimeType.JSON);
}

function shareWithBot_(fileId, shareTo) {
  if (!shareTo) return;
  DriveApp.getFileById(fileId).addEditor(shareTo);
}

function createSlides_(parameters) {
  var title = parameters.title || 'Untitled';
  var presentation = SlidesApp.create(title);
  var id = presentation.getId();
  shareWithBot_(id, parameters.share_to);
  return {
    id: id,
    url: 'https://docs.google.com/presentation/d/' + id + '/edit',
  };
}

function createDoc_(parameters) {
  var title = parameters.title || 'Untitled';
  var document = DocumentApp.create(title);
  var id = document.getId();
  shareWithBot_(id, parameters.share_to);
  return {
    id: id,
    url: 'https://docs.google.com/document/d/' + id + '/edit',
  };
}

function createSheet_(parameters) {
  var title = parameters.title || 'Untitled';
  var spreadsheet = SpreadsheetApp.create(title);
  var id = spreadsheet.getId();
  shareWithBot_(id, parameters.share_to);
  return {
    id: id,
    url: 'https://docs.google.com/spreadsheets/d/' + id + '/edit',
  };
}

function createCalendarEvent_(parameters) {
  if (!parameters.title) throw new Error('title is required');
  if (!parameters.start) throw new Error('start (ISO 8601) is required');
  if (!parameters.end) throw new Error('end (ISO 8601) is required');
  var attendees = parameters.attendees
    ? String(parameters.attendees).split(',').map(function (email) { return email.trim(); }).filter(Boolean)
    : [];
  var event = CalendarApp.getDefaultCalendar().createEvent(
    parameters.title,
    new Date(parameters.start),
    new Date(parameters.end),
    {guests: attendees.join(',')}
  );
  return {id: event.getId()};
}

function sendGmail_(parameters) {
  if (!parameters.to) throw new Error('to is required');
  if (!parameters.subject) throw new Error('subject is required');
  GmailApp.sendEmail(parameters.to, parameters.subject, parameters.body || '');
  return {sent: true};
}
