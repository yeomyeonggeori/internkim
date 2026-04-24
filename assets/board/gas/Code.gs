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
      case 'calendar.list':
        return reply_(listCalendarEvents_(parameters));
      case 'gmail.send':
        return reply_(sendGmail_(parameters));
      case 'drive.import_pptx':
        return reply_(importPptxAsSlides_(parameters));
      case 'drive.import_docx':
        return reply_(importDocxAsDoc_(parameters));
      case 'drive.import_xlsx':
        return reply_(importXlsxAsSheet_(parameters));
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
  var options = {};
  if (attendees.length) options.guests = attendees.join(',');
  if (parameters.description) options.description = String(parameters.description);
  if (parameters.location) options.location = String(parameters.location);
  var event = CalendarApp.getDefaultCalendar().createEvent(
    parameters.title,
    new Date(parameters.start),
    new Date(parameters.end),
    options
  );
  var id = event.getId();
  var calendarId = CalendarApp.getDefaultCalendar().getId();
  var eid = Utilities.base64EncodeWebSafe(id.replace('@google.com', '') + ' ' + calendarId).replace(/=+$/, '');
  return {
    id: id,
    url: 'https://www.google.com/calendar/event?eid=' + eid,
  };
}

function listCalendarEvents_(parameters) {
  var now = new Date();
  var start = parameters.start ? new Date(parameters.start) : now;
  var end = parameters.end
    ? new Date(parameters.end)
    : new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000);
  var calendar = CalendarApp.getDefaultCalendar();
  var events = calendar.getEvents(start, end);
  var max = parameters.limit ? parseInt(parameters.limit, 10) : 50;
  return {
    calendarId: calendar.getId(),
    range: {start: start.toISOString(), end: end.toISOString()},
    events: events.slice(0, max).map(function (e) {
      return {
        id: e.getId(),
        title: e.getTitle(),
        start: e.getStartTime().toISOString(),
        end: e.getEndTime().toISOString(),
        location: e.getLocation() || '',
        description: e.getDescription() || '',
        allDay: e.isAllDayEvent(),
      };
    }),
  };
}

function sendGmail_(parameters) {
  if (!parameters.to) throw new Error('to is required');
  if (!parameters.subject) throw new Error('subject is required');
  GmailApp.sendEmail(parameters.to, parameters.subject, parameters.body || '');
  return {sent: true};
}

/**
 * Upload a base64-encoded Office document to the user's Drive and have
 * Google convert it to its native equivalent (Slides / Docs / Sheets)
 * in a single step. Uses the Drive v3 multipart upload API via
 * UrlFetchApp so no Advanced Drive Service setup is needed in the
 * script project — just the drive.file scope the bridge already has.
 */
function importOfficeFile_(parameters, sourceMimeType, targetMimeType, urlPrefix) {
  if (!parameters.data) throw new Error('data (base64) is required');
  var title = parameters.title || 'Imported file';
  var sourceBytes = Utilities.base64Decode(parameters.data);
  var metadata = {
    name: title,
    mimeType: targetMimeType,
  };
  var boundary = '-------314159265358979323846';
  var delimiter = '\r\n--' + boundary + '\r\n';
  var closeDelimiter = '\r\n--' + boundary + '--';
  var multipartBody =
    delimiter +
    'Content-Type: application/json; charset=UTF-8\r\n\r\n' +
    JSON.stringify(metadata) +
    delimiter +
    'Content-Type: ' + sourceMimeType + '\r\n' +
    'Content-Transfer-Encoding: base64\r\n\r\n' +
    parameters.data +
    closeDelimiter;
  var response = UrlFetchApp.fetch(
    'https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&supportsAllDrives=true',
    {
      method: 'post',
      contentType: 'multipart/related; boundary=' + boundary,
      headers: {Authorization: 'Bearer ' + ScriptApp.getOAuthToken()},
      payload: multipartBody,
      muteHttpExceptions: true,
    }
  );
  if (response.getResponseCode() !== 200) {
    throw new Error('Drive upload failed: ' + response.getContentText());
  }
  var result = JSON.parse(response.getContentText());
  var id = result.id;
  shareWithBot_(id, parameters.share_to);
  return {id: id, url: urlPrefix + id + '/edit'};
}

function importPptxAsSlides_(parameters) {
  return importOfficeFile_(
    parameters,
    'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    'application/vnd.google-apps.presentation',
    'https://docs.google.com/presentation/d/'
  );
}

function importDocxAsDoc_(parameters) {
  return importOfficeFile_(
    parameters,
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    'application/vnd.google-apps.document',
    'https://docs.google.com/document/d/'
  );
}

function importXlsxAsSheet_(parameters) {
  return importOfficeFile_(
    parameters,
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    'application/vnd.google-apps.spreadsheet',
    'https://docs.google.com/spreadsheets/d/'
  );
}
