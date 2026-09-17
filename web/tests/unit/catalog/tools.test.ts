import { describe, expect, test } from 'bun:test';
import { z } from 'zod';

import {
  ArtifactKind,
  ArtifactToolName,
  BrowserToolName,
  CalendarToolName,
  DocumentToolName,
  ImageToolName,
  MessageAuthor,
  MessageDeliveryStatus,
  MessageSearchScope,
  MessageTargetType,
  MessageToolName,
  SiteLifecycleStatus,
  SiteServeMode,
  SiteToolName,
  WebToolName,
  WorkspaceTaskInitialStatus,
  WorkspaceTaskSize,
  artifactReviewInputSchema,
  artifactReviewResultSchema,
  browserClickInputSchema,
  browserClickResultSchema,
  browserOpenInputSchema,
  browserOpenResultSchema,
  browserScreenshotInputSchema,
  browserScreenshotResultSchema,
  browserSnapshotInputSchema,
  browserSnapshotResultSchema,
  buildCapabilityToolCatalog,
  calendarAddInputSchema,
  capabilityToolResultSchema,
  calendarDeleteInputSchema,
  calendarDeleteInputIntentSchema,
  calendarListInputSchema,
  calendarUpdateInputSchema,
  calendarUpdateInputIntentSchema,
  documentReadInputSchema,
  documentReadResultSchema,
  imageReadInputSchema,
  imageReadResultSchema,
  messageContextInputSchema,
  messageContextResultSchema,
  messageDeleteInputSchema,
  messageDeleteResultSchema,
  messageSearchInputSchema,
  messageSearchResultSchema,
  messageSendInputSchema,
  messageSendResultSchema,
  messageUpdateInputSchema,
  messageUpdateResultSchema,
  siteListInputSchema,
  siteListResultSchema,
  siteServeInputSchema,
  siteServeInputIntentSchema,
  siteServeResultSchema,
  siteUnserveInputSchema,
  siteUnserveResultSchema,
  leaveGrantSetInputSchema,
  leaveReturnEarlyInputSchema,
  taskAddInputSchema,
  taskDeleteInputSchema,
  taskDeleteInputIntentSchema,
  taskListInputSchema,
  taskUpdateInputSchema,
  taskUpdateInputIntentSchema,
  webFetchResultSchema,
  webSearchInputSchema,
  webSearchResultSchema,
} from '../../../src/lib/server/public-api/catalog/tools';
import {
  CapabilityAnsweredBy,
  CapabilityModelVisibility,
  CapabilitySideEffect,
  ResourceEffectIdentity,
  protocolVersion,
} from '../../../src/lib/server/public-api/catalog/protocol';

describe('canonical capability tools', () => {
  test('offers exactly this roster', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);

    expect(catalog.tools.map(tool => tool.name).sort()).toEqual([
      'artifact_review',
      'attendance_add',
      'attendance_delete',
      'attendance_leave_policy_get',
      'attendance_leave_policy_set',
      'attendance_list',
      'attendance_update',
      'attendance_work_policy_get',
      'attendance_work_policy_set',
      'attention_triage',
      'browser_click',
      'browser_fill',
      'browser_handoff',
      'browser_open',
      'browser_press',
      'browser_screenshot',
      'browser_select',
      'browser_snapshot',
      'browser_wait',
      'company_document_download',
      'company_document_list',
      'company_document_register',
      'company_document_search',
      'company_document_update',
      'company_document_upload',
      'company_holiday_add',
      'company_holiday_delete',
      'company_holiday_list',
      'company_holiday_update',
      'company_info_get',
      'company_info_set',
      'company_metric_list',
      'company_metric_record',
      'company_record_add',
      'company_record_delete',
      'company_record_list',
      'company_record_update',
      'company_settings_get',
      'company_settings_update',
      'conversation_mute',
      'conversation_unmute',
      'crm_activity_list',
      'crm_activity_save',
      'crm_contact_add',
      'crm_contact_archive',
      'crm_contact_list',
      'crm_contact_update',
      'crm_opportunity_add',
      'crm_opportunity_archive',
      'crm_opportunity_list',
      'crm_opportunity_move',
      'crm_opportunity_update',
      'crm_organization_add',
      'crm_organization_archive',
      'crm_organization_list',
      'crm_organization_update',
      'crm_vocabulary_get',
      'crm_vocabulary_set',
      'document_read',
      'embedding_create',
      'event_add',
      'event_delete',
      'event_list',
      'event_update',
      'image_generate',
      'image_read',
      'leave_balance',
      'leave_decide',
      'leave_delete',
      'leave_grant_set',
      'leave_list',
      'leave_request',
      'leave_return_early',
      'leave_update',
      'llm_structured',
      'llm_text',
      'mail_connection_start',
      'mail_connection_status',
      'mail_message_list',
      'mail_message_mark',
      'mail_message_move',
      'mail_message_read',
      'mail_message_search',
      'mail_message_send',
      'message_context',
      'message_delete',
      'message_search',
      'message_send',
      'message_update',
      'notification_settings_get',
      'notification_settings_set',
      'person_invite',
      'person_list',
      'person_update',
      'push_device_claim',
      'push_device_release',
      'push_reachability_get',
      'site_list',
      'site_serve',
      'site_unserve',
      'task_add',
      'task_delete',
      'task_list',
      'task_update',
      'task_vocabulary_set',
      'team_add',
      'team_delete',
      'team_list',
      'team_update',
      'web_fetch',
      'web_search',
    ]);
  });

  test('publishes every tool exactly once, strictly, under this protocol version', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);

    expect(catalog.protocolVersion).toBe(protocolVersion);
    expect(catalog.tools.length).toBeGreaterThan(0);
    expect(new Set(catalog.tools.map(tool => tool.name)).size).toBe(catalog.tools.length);
    expect(catalog.tools.every(tool => tool.inputSchemaStrict && tool.outputSchemaStrict)).toBe(true);
    expect(catalog.tools.every(tool => tool.name === tool.canonicalName && tool.name === tool.modelName)).toBe(true);
  });

  // A tool the model can call promises a shape back, so it carries a contract. A
  // tool carrying none answers in the invoke envelope and is never shown to the
  // model, because there would be nothing to tell it about the answer.
  test('never shows the model a tool whose answer has no shape', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);

    for (const tool of catalog.tools) {
      if (tool.resultContract === undefined) {
        expect(tool.modelVisible).toBe(false);
        continue;
      }
      expect(JSON.stringify(tool.outputSchema)).toBe(JSON.stringify(tool.resultContract.schema));
    }
    expect(catalog.tools.every(tool => !tool.modelVisible || tool.resultContract !== undefined)).toBe(true);
  });

  test('names the answerer of every tool', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const answerers = Object.values(CapabilityAnsweredBy);

    expect(catalog.tools.every(tool => answerers.includes(tool.answeredBy))).toBe(true);
    expect(catalog.tools.filter(tool => tool.answeredBy === CapabilityAnsweredBy.Record).length).toBeGreaterThan(0);
    expect(catalog.tools.filter(tool => tool.answeredBy === CapabilityAnsweredBy.Company).length).toBeGreaterThan(0);
    expect(catalog.tools.filter(tool => tool.answeredBy === CapabilityAnsweredBy.Local).length).toBeGreaterThan(0);
  });

  // A browser tool drives one session on one machine, so it is the requester's
  // to hold and a grant's to gate, whichever browser answers it.
  test('gives every browser tool the requester and a grant', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const browserTools = catalog.tools.filter(tool => tool.namespace === 'browser');

    expect(browserTools.length).toBeGreaterThan(0);
    for (const tool of browserTools) {
      expect(tool.answeredBy).toBe(CapabilityAnsweredBy.Local);
      expect(tool.requiresRequesterDevice).toBe(true);
      expect(tool.approvalScope).toBe('browser');
    }
  });

  test('publishes explicit intent schemas for model-visible state changes', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const stateChangingTools = catalog.tools.filter(tool =>
      tool.modelVisibility === CapabilityModelVisibility.Visible
      && tool.sideEffectClass !== CapabilitySideEffect.Read
      && tool.sideEffectClass !== CapabilitySideEffect.Computation
    );

    expect(stateChangingTools.map(tool => tool.name)).toEqual([
      'task_add',
      'task_update',
      'task_delete',
      'task_vocabulary_set',
      'person_update',
      'person_invite',
      'team_add',
      'team_update',
      'team_delete',
      CalendarToolName.Add,
      CalendarToolName.Update,
      CalendarToolName.Delete,
      'leave_request',
      'leave_update',
      'leave_delete',
      'leave_decide',
      'leave_grant_set',
      'leave_return_early',
      'attendance_add',
      'attendance_update',
      'attendance_delete',
      MessageToolName.Send,
      MessageToolName.Update,
      MessageToolName.Delete,
      SiteToolName.Serve,
      SiteToolName.Unserve,
      BrowserToolName.Open,
      BrowserToolName.Click,
      'company_document_register',
      'company_document_update',
      'company_document_upload',
      'company_info_set',
      'company_metric_record',
      'company_record_add',
      'company_record_delete',
      'company_record_update',
      'crm_organization_add',
      'crm_organization_update',
      'crm_organization_archive',
      'crm_contact_add',
      'crm_contact_update',
      'crm_contact_archive',
      'crm_opportunity_add',
      'crm_opportunity_update',
      'crm_opportunity_move',
      'crm_opportunity_archive',
      'crm_activity_save',
      'crm_vocabulary_set',
      'company_settings_update',
      'company_holiday_add',
      'company_holiday_update',
      'company_holiday_delete',
      'attendance_work_policy_set',
      'attendance_leave_policy_set',
      'notification_settings_set',
      'conversation_mute',
      'conversation_unmute',
      'mail_connection_start',
      'mail_message_send',
    ]);

    for (const tool of stateChangingTools) {
      if (tool.inputIntentSchema === undefined) {
        throw new Error(`${tool.name} is missing inputIntentSchema`);
      }
      const intentSchema = z.fromJSONSchema(tool.inputIntentSchema);
      expect(intentSchema.safeParse({}).success).toBe(true);
      expect(intentSchema.safeParse({ unexpected: true }).success).toBe(false);
    }

    expect(siteServeInputIntentSchema.safeParse({ title: 'Team Dashboard' }).success).toBe(true);
    expect(siteServeInputIntentSchema.safeParse({ mode: 'publish', unexpected: true }).success).toBe(false);
  });

  test('defines exact web search inputs and normalized results', () => {
    expect(webSearchInputSchema.safeParse({ query: 'internkim', limit: 3 }).success).toBe(true);
    expect(webSearchInputSchema.safeParse({ query: 'internkim', allowedDomains: ['internkim.example'] }).success).toBe(true);
    expect(webSearchInputSchema.safeParse({ query: 'internkim', limit: 0 }).success).toBe(false);
    expect(webSearchInputSchema.safeParse({ query: '   ' }).success).toBe(false);
    expect(webSearchInputSchema.safeParse({ query: 'internkim', unknown: true }).success).toBe(false);
    expect(webSearchInputSchema.safeParse({}).success).toBe(false);

    const result = {
      provider: 'openrouter',
      remoteLLMInvolved: true,
      compatibility: 'openrouter_server_tool_auto',
      query: 'internkim',
      answer: 'result',
      results: [{
        title: 'InternKim',
        url: 'https://internkim.example',
        snippet: 'An agent platform',
        source: 'internkim.example',
      }],
    };
    expect(webSearchResultSchema.safeParse(result).success).toBe(true);
    expect(webSearchResultSchema.safeParse({ ...result, extra: true }).success).toBe(false);
    expect(webSearchResultSchema.safeParse({ ...result, results: [{ title: 'InternKim' }] }).success).toBe(false);

    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const descriptor = catalog.tools.find(tool => tool.name === WebToolName.Search);
    expect(descriptor?.resultContract?.effects).toEqual([]);
    expect(descriptor?.requiresApproval).toBeUndefined();
    expect(descriptor?.sideEffectClass).toBe(CapabilitySideEffect.Read);
  });

  test('defines the normalized web fetch result', () => {
    const result = {
      provider: 'openrouter',
      remoteLLMInvolved: true,
      compatibility: 'openrouter_server_tool_auto',
      results: [{ url: 'https://example.com', finalURL: 'https://example.com', title: 'Example', content: 'Page text' }],
      errors: [{ url: 'https://blocked.example', error: 'blocked' }],
    };
    expect(webFetchResultSchema.safeParse(result).success).toBe(true);
    expect(webFetchResultSchema.safeParse({ ...result, errors: undefined }).success).toBe(false);
    expect(webFetchResultSchema.safeParse({ ...result, results: [{ url: 'https://example.com', title: 'Example' }] }).success).toBe(false);

    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const descriptor = catalog.tools.find(tool => tool.name === WebToolName.Fetch);
    expect(descriptor?.modelVisible).toBe(true);
    expect(descriptor?.resultContract?.effects).toEqual([]);
    expect(descriptor?.sideEffectClass).toBe(CapabilitySideEffect.Read);
  });

  test('shows the model the mail tools the mail skill calls, with the answers admind gives', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const visibleMailToolNames = catalog.tools
      .filter(tool => tool.namespace === 'mail' && tool.modelVisible)
      .map(tool => tool.name);
    expect(visibleMailToolNames).toEqual([
      'mail_connection_start',
      'mail_connection_status',
      'mail_message_list',
      'mail_message_read',
      'mail_message_search',
      'mail_message_send',
    ]);

    const message = { uid: 42, mailbox: 'INBOX', subject: 'Invoice', from: 'alice@example.com', date: '2026-09-17T09:00:00Z', preview: 'Attached', isRead: false };
    expect(capabilityToolResultSchema('mail_message_list')?.safeParse({ messages: [message], nextCursor: '' }).success).toBe(true);
    expect(capabilityToolResultSchema('mail_message_search')?.safeParse({ messages: null, nextCursor: '' }).success).toBe(false);
    expect(capabilityToolResultSchema('mail_message_read')?.safeParse({
      uid: 42, mailbox: 'INBOX', subject: 'Invoice', from: 'alice@example.com', to: 'me@example.com', cc: '', date: '2026-09-17T09:00:00Z', body: 'Hello', isRead: true,
    }).success).toBe(true);
    expect(capabilityToolResultSchema('mail_message_send')?.safeParse({ sent: true, appendedTo: 'Sent' }).success).toBe(true);
    expect(capabilityToolResultSchema('mail_message_send')?.safeParse({ sent: false }).success).toBe(false);
    expect(capabilityToolResultSchema('mail_connection_start')?.safeParse({
      status: 'configuration_required', provider: 'manual', setupURL: 'http://admind.local/mail/',
    }).success).toBe(true);

    const sendTool = catalog.tools.find(tool => tool.name === 'mail_message_send');
    expect(sendTool?.resultContract?.effects).toEqual([]);
    expect(sendTool?.completionEvidence).toEqual({ mode: 'success', action: 'send_email', targetKind: 'email' });
    expect(catalog.tools.find(tool => tool.name === 'mail_message_mark')?.modelVisible).toBe(false);
    expect(catalog.tools.find(tool => tool.name === 'mail_message_move')?.modelVisible).toBe(false);
  });

  test('defines exact browser inputs and successful results', () => {
    expect(browserOpenInputSchema.safeParse({ url: 'https://preview.example/site-1' }).success).toBe(true);
    expect(browserSnapshotInputSchema.safeParse({}).success).toBe(true);
    expect(browserScreenshotInputSchema.safeParse({ ttlSeconds: 300 }).success).toBe(true);
    expect(browserClickInputSchema.safeParse({ ref: '@e1' }).success).toBe(true);

    expect(browserOpenInputSchema.safeParse({ startURL: 'https://preview.example/site-1' }).success).toBe(false);
    expect(browserSnapshotInputSchema.safeParse({ interactive: true }).success).toBe(false);
    expect(browserScreenshotInputSchema.safeParse({ ttlSeconds: -1 }).success).toBe(false);
    expect(browserClickInputSchema.safeParse({}).success).toBe(false);

    expect(browserOpenResultSchema.safeParse({
      url: 'https://preview.example/site-1',
      requestedURL: 'https://preview.example/site-1',
      title: 'Quarterly support',
      snapshotText: '- button "Open report" [ref=e1]',
      interactiveRefs: ['@e1'],
      capturedAt: '2026-07-19T00:00:00Z',
    }).success).toBe(true);
    expect(browserSnapshotResultSchema.safeParse({
      url: 'https://preview.example/site-1',
      title: 'Quarterly support',
      snapshotText: '- button "Open report" [ref=e1]',
      interactiveRefs: ['@e1'],
      hasMore: false,
      capturedAt: '2026-07-19T00:00:00Z',
    }).success).toBe(true);
    expect(browserScreenshotResultSchema.safeParse({
      fileID: 'file-1',
      filename: 'site.png',
      sizeBytes: 1024,
      contentType: 'image/png',
      devicePath: '/tmp/internkim-companion-files/site.png',
      expiresAt: '2026-07-19T00:05:00Z',
      capturedAt: '2026-07-19T00:00:00Z',
    }).success).toBe(true);
    expect(browserClickResultSchema.safeParse({
      ok: true,
      action: 'click',
      target: '@e1',
      capturedAt: '2026-07-19T00:00:00Z',
    }).success).toBe(true);
    expect(browserClickResultSchema.safeParse({
      ok: true,
      action: 'fill',
      capturedAt: '2026-07-19T00:00:00Z',
    }).success).toBe(false);
  });

  test('defines exact artifact review evidence and result contracts', () => {
    const reviewInput = {
      artifactKind: ArtifactKind.Site,
      intent: 'Check the customer support landing page',
      rubric: 'Verify hierarchy, text fit, and primary interaction',
      evidence: [{
        role: 'desktopScreenshot',
        path: '/tmp/internkim-companion-files/site.png',
        mimeType: 'image/png',
        label: 'Desktop preview',
      }],
    };
    const reviewResult = {
      passed: false,
      issues: [{
        severity: 'warning',
        category: 'visualHierarchy',
        target: 'Primary action',
        message: 'The action is hard to distinguish.',
        suggestedFix: 'Increase contrast.',
      }],
      acceptedWarnings: [],
      summary: 'One visual hierarchy issue remains.',
    };

    expect(artifactReviewInputSchema.safeParse(reviewInput).success).toBe(true);
    expect(artifactReviewResultSchema.safeParse(reviewResult).success).toBe(true);
    expect(artifactReviewInputSchema.safeParse({ ...reviewInput, evidence: [] }).success).toBe(false);
    expect(artifactReviewInputSchema.safeParse({
      ...reviewInput,
      evidence: [{ ...reviewInput.evidence[0], mimeType: 'text/html' }],
    }).success).toBe(false);
    expect(artifactReviewResultSchema.safeParse({
      ...reviewResult,
      issues: [{ ...reviewResult.issues[0], category: 'performance' }],
    }).success).toBe(false);

    const catalog = buildCapabilityToolCatalog(protocolVersion);
    for (const toolName of [
      BrowserToolName.Open,
      BrowserToolName.Snapshot,
      BrowserToolName.Screenshot,
      BrowserToolName.Click,
      ArtifactToolName.Review,
    ]) {
      expect(catalog.tools.find(tool => tool.name === toolName)?.resultContract?.effects).toEqual([]);
    }
    expect(catalog.tools.find(tool => tool.name === ArtifactToolName.Review)?.resultContract?.evidenceCondition).toEqual({
      resultField: 'passed',
      equals: true,
    });
  });

  test('keeps task mutation contracts exact', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const addTool = catalog.tools.find(tool => tool.name === 'task_add');
    const updateTool = catalog.tools.find(tool => tool.name === 'task_update');
    const deleteTool = catalog.tools.find(tool => tool.name === 'task_delete');

    expect(addTool?.resultContract?.effects).toEqual([
      { objectType: 'task', effect: 'created', resultField: 'taskID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(updateTool?.resultContract?.effects).toEqual([
      { objectType: 'task', effect: 'updated', resultField: 'taskID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(deleteTool?.resultContract?.effects).toEqual([
      { objectType: 'task', effect: 'deleted', resultField: 'taskID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(deleteTool?.requiresApproval).toBe(true);
  });

  test('keeps runtime task identities out of user intent', () => {
    expect(taskUpdateInputIntentSchema.safeParse({ title: 'quarterly settlement review' }).success).toBe(true);
    expect(taskUpdateInputIntentSchema.safeParse({ taskHint: 'task-1' }).success).toBe(false);
    expect(taskDeleteInputIntentSchema.safeParse({}).success).toBe(true);
    expect(taskDeleteInputIntentSchema.safeParse({ taskHint: 'task-1' }).success).toBe(false);
  });

  test('keeps calendar metadata and mutation contracts explicit', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const addTool = catalog.tools.find(tool => tool.name === CalendarToolName.Add);
    const listTool = catalog.tools.find(tool => tool.name === CalendarToolName.List);
    const updateTool = catalog.tools.find(tool => tool.name === CalendarToolName.Update);
    const deleteTool = catalog.tools.find(tool => tool.name === CalendarToolName.Delete);

    expect(addTool).toMatchObject({
      namespace: 'calendar',
      privacyClass: 'workspace_calendar',
      policyResource: 'tool:event_add',
    });
    expect(addTool?.resultContract?.effects).toEqual([
      { objectType: 'calendar', effect: 'created', resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(listTool?.resultContract?.effects).toEqual([]);
    expect(updateTool?.resultContract?.effects).toEqual([
      { objectType: 'calendar', effect: 'updated', resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(deleteTool?.resultContract?.effects).toEqual([
      { objectType: 'calendar', effect: 'deleted', resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(deleteTool?.requiresApproval).toBe(true);
  });

  test('keeps runtime calendar identities out of user intent', () => {
    expect(calendarUpdateInputIntentSchema.safeParse({ startsAt: '2026-07-24T15:00:00+09:00' }).success).toBe(true);
    expect(calendarUpdateInputIntentSchema.safeParse({ eventHint: 'event-1' }).success).toBe(false);
    expect(calendarDeleteInputIntentSchema.safeParse({}).success).toBe(true);
    expect(calendarDeleteInputIntentSchema.safeParse({ eventHint: 'event-1' }).success).toBe(false);
  });

  test('validates shallow message inputs', () => {
    expect(messageContextInputSchema.safeParse({}).success).toBe(true);
    expect(messageSearchInputSchema.safeParse({
      scope: MessageSearchScope.CurrentChannel,
      authoredBy: MessageAuthor.Assistant,
      queries: ['quarterly settlement'],
      limit: 20,
    }).success).toBe(true);
    expect(messageSendInputSchema.safeParse({
      targetType: MessageTargetType.DirectMessage,
      message: 'please review the quarterly settlement material.',
      personHint: '@support-lead',
    }).success).toBe(true);
    expect(messageUpdateInputSchema.safeParse({
      messageID: 'message-1',
      oldText: 'quarterly settlement material',
      newText: 'quarterly settlement notice',
    }).success).toBe(true);
    expect(messageDeleteInputSchema.safeParse({
      messageIDs: ['message-1', 'message-2'],
    }).success).toBe(true);

    expect(messageContextInputSchema.safeParse({ scope: 'currentChannel' }).success).toBe(false);
    expect(messageSearchInputSchema.safeParse({ query: 'quarterly settlement' }).success).toBe(false);
    expect(messageSearchInputSchema.safeParse({ limit: 26 }).success).toBe(false);
    expect(messageSendInputSchema.safeParse({
      targetType: MessageTargetType.CurrentChannel,
      message: '   ',
    }).success).toBe(false);
    expect(messageSendInputSchema.safeParse({
      targetType: MessageTargetType.CurrentChannel,
      message: 'notice',
      deliveryTarget: { type: 'currentChannel' },
    }).success).toBe(false);
    expect(messageUpdateInputSchema.safeParse({ messageID: 'message-1' }).success).toBe(false);
    expect(messageDeleteInputSchema.safeParse({ messageIDs: [] }).success).toBe(false);
    expect(messageDeleteInputSchema.safeParse({ messageIDs: ['message-1', 'message-1'] }).success).toBe(false);
  });

  test('requires canonical message result identities', () => {
    const contextResult = {
      platform: 'mattermost',
      conversationID: 'conversation-1',
      conversationType: 'direct',
      channelID: 'channel-1',
      channelName: 'support',
      replyTargetID: 'message-1',
      rootMessageID: '',
      currentMessageID: 'message-1',
      requesterPersonID: 'person-1',
      requesterPlatformUserID: 'user-1',
      botUserID: 'bot-1',
      botUsername: 'internkim',
    };
    const searchResult = {
      scope: MessageSearchScope.CurrentChannel,
      queries: ['quarterly settlement'],
      authoredBy: MessageAuthor.Assistant,
      messageIDs: ['message-1'],
      candidates: [{
        messageID: 'message-1',
        channelID: 'channel-1',
        userID: 'bot-1',
        authoredBy: MessageAuthor.Assistant,
        createdAt: 1784422800000,
        preview: 'quarterly settlement notice',
        deletable: true,
      }],
      hasMore: false,
    };
    const sendResult = {
      messageIDs: ['message-2'],
      deliveryStatus: MessageDeliveryStatus.Sent,
    };
    const updateResult = {
      messageID: 'message-2',
      deliveryStatus: MessageDeliveryStatus.Updated,
      messageUpdated: true,
      isPinned: false,
    };
    const deleteResult = {
      messageIDs: ['message-2'],
      deliveryStatus: MessageDeliveryStatus.Deleted,
    };

    expect(messageContextResultSchema.safeParse(contextResult).success).toBe(true);
    expect(messageSearchResultSchema.safeParse(searchResult).success).toBe(true);
    expect(messageSendResultSchema.safeParse(sendResult).success).toBe(true);
    expect(messageUpdateResultSchema.safeParse(updateResult).success).toBe(true);
    expect(messageDeleteResultSchema.safeParse(deleteResult).success).toBe(true);

    expect(messageContextResultSchema.safeParse({ ...contextResult, extra: true }).success).toBe(false);
    expect(messageSearchResultSchema.safeParse({ ...searchResult, candidates: [{ messageID: 'message-1' }] }).success).toBe(false);
    expect(messageSendResultSchema.safeParse({ ...sendResult, messageIDs: [] }).success).toBe(false);
    expect(messageUpdateResultSchema.safeParse({ ...updateResult, messageID: '' }).success).toBe(false);
    expect(messageDeleteResultSchema.safeParse({ ...deleteResult, messageIDs: ['message-2', 'message-2'] }).success).toBe(false);
  });

  test('publishes exact message effects and approvals', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const contextTool = catalog.tools.find(tool => tool.name === MessageToolName.Context);
    const searchTool = catalog.tools.find(tool => tool.name === MessageToolName.Search);
    const sendTool = catalog.tools.find(tool => tool.name === MessageToolName.Send);
    const updateTool = catalog.tools.find(tool => tool.name === MessageToolName.Update);
    const deleteTool = catalog.tools.find(tool => tool.name === MessageToolName.Delete);

    expect(contextTool?.resultContract?.effects).toEqual([]);
    expect(searchTool?.resultContract?.effects).toEqual([]);
    expect(sendTool?.resultContract?.effects).toEqual([
      { objectType: 'message', effect: 'sent', resultField: 'messageIDs', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(updateTool?.resultContract?.effects).toEqual([
      { objectType: 'message', effect: 'updated', resultField: 'messageID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(deleteTool?.resultContract?.effects).toEqual([
      { objectType: 'message', effect: 'deleted', resultField: 'messageIDs', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(contextTool?.requiresApproval).toBeUndefined();
    expect(searchTool?.requiresApproval).toBeUndefined();
    expect(sendTool?.requiresApproval).toBe(true);
    expect(updateTool?.requiresApproval).toBe(false);
    expect(deleteTool?.requiresApproval).toBe(true);
    expect(sendTool?.idempotency).toEqual({ supported: true, required: false, scope: 'operation' });
    expect(updateTool?.idempotency).toEqual({ supported: false, required: false, scope: 'operation' });
    expect(sendTool?.completionEvidence).toEqual({
      mode: 'success',
      action: 'send_message',
      targetKind: 'message',
    });
  });

  test('validates task inputs without operation aliases', () => {
    expect(taskAddInputSchema.parse({
      title: 'customer support quarterly settlement gap check',
      size: WorkspaceTaskSize.Small,
      status: WorkspaceTaskInitialStatus.planned,
      endsAt: '2026-07-24',
    })).toEqual({
      title: 'customer support quarterly settlement gap check',
      size: WorkspaceTaskSize.Small,
      status: WorkspaceTaskInitialStatus.planned,
      endsAt: '2026-07-24',
    });
    expect(taskListInputSchema.safeParse({ query: 'settlement', scope: 'self' }).success).toBe(true);
    expect(taskUpdateInputSchema.safeParse({ taskHint: 'task-1', title: 'updated title' }).success).toBe(true);
    expect(taskDeleteInputSchema.safeParse({ taskHint: 'task-1' }).success).toBe(true);

    expect(taskAddInputSchema.safeParse({ content: 'invalid alias' }).success).toBe(false);
    expect(taskUpdateInputSchema.safeParse({ taskHint: 'task-1' }).success).toBe(false);
    expect(taskUpdateInputSchema.safeParse({ query: 'settlement', content: 'update' }).success).toBe(false);
    expect(taskDeleteInputSchema.safeParse({ taskHint: 'task-1', query: 'settlement' }).success).toBe(false);
  });

  test('validates calendar inputs with exact mutation identities', () => {
    expect(calendarAddInputSchema.safeParse({
      title: 'customer support weekly check',
      startsAt: '2026-07-24T14:00:00+09:00',
      endsAt: '2026-07-24T15:00:00+09:00',
      participantPersonHints: ['support@example.com'],
    }).success).toBe(true);
    expect(calendarUpdateInputSchema.safeParse({
      eventHint: 'event-1',
      startsAt: '2026-07-24T15:00:00+09:00',
    }).success).toBe(true);
    expect(calendarDeleteInputSchema.safeParse({ eventHint: 'event-1' }).success).toBe(true);
    expect(calendarListInputSchema.safeParse({ limit: 2 }).success).toBe(true);

    expect(calendarUpdateInputSchema.safeParse({ eventHint: 'event-1', notifyMinutesBefore: 0 }).success).toBe(true);

    expect(calendarUpdateInputSchema.safeParse({ eventHint: 'event-1' }).success).toBe(false);
    expect(calendarAddInputSchema.safeParse({
      title: 'customer support weekly check',
      startsAt: '2026-07-24T14:00:00+09:00',
      endsAt: '2026-07-24T15:00:00+09:00',
      notifyMinutesBefore: -30,
    }).success).toBe(false);
    expect(calendarUpdateInputSchema.safeParse({ query: 'weekly check', title: 'change' }).success).toBe(false);
    expect(calendarDeleteInputSchema.safeParse({ query: 'weekly check' }).success).toBe(false);
    expect(calendarListInputSchema.safeParse({ limit: 0 }).success).toBe(false);
    expect(calendarListInputSchema.safeParse({ limit: 1.5 }).success).toBe(false);
  });

  test('publishes provider-portable minimum mutation property counts', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const taskUpdateTool = catalog.tools.find(tool => tool.name === 'task_update');
    const calendarAddTool = catalog.tools.find(tool => tool.name === CalendarToolName.Add);
    const calendarUpdateTool = catalog.tools.find(tool => tool.name === CalendarToolName.Update);

    expect(taskUpdateTool?.inputSchema).toMatchObject({ minProperties: 2 });
    expect(calendarAddTool?.inputSchema).toMatchObject({
      properties: {
        notifyMinutesBefore: {
          type: 'integer',
        },
      },
    });
    expect(calendarUpdateTool?.inputSchema).toMatchObject({ minProperties: 2 });
  });

  test('keeps the hosting boundary to serve, list, and unserve', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const siteTools = catalog.tools.filter(tool => tool.namespace === 'site');

    expect(siteTools.map(tool => tool.name)).toEqual([
      SiteToolName.Serve,
      SiteToolName.List,
      SiteToolName.Unserve,
    ]);
    expect(siteTools.every(tool => tool.resultContract !== undefined)).toBe(true);
    expect(siteTools.every(tool => tool.resultContract?.schema.additionalProperties === false)).toBe(true);
  });

  test('requires exact site identities for hosting mutations', () => {
    expect(siteServeInputSchema.safeParse({
      title: 'customer support quarterly settlement',
      sourceWorkspacePath: '~/sites/customer-support-quarterly',
      mode: SiteServeMode.Publish,
    }).success).toBe(true);
    expect(siteServeInputSchema.safeParse({
      title: 'Customer Support Quarterly',
      sourceWorkspacePath: '~/sites/customer-support-quarterly',
      mode: SiteServeMode.Preview,
      siteReference: 'customer-support-quarterly',
    }).success).toBe(true);
    expect(siteListInputSchema.safeParse({}).success).toBe(true);
    expect(siteListInputSchema.safeParse({ siteReference: 'site-1' }).success).toBe(true);
    expect(siteUnserveInputSchema.safeParse({ siteReference: 'site-1', reason: 'The campaign ended.' }).success).toBe(true);

    expect(siteServeInputSchema.safeParse({ title: '', sourceWorkspacePath: '~/sites/a', mode: 'publish' }).success).toBe(false);
    expect(siteServeInputSchema.safeParse({ title: 'A', mode: 'publish' }).success).toBe(false);
    expect(siteServeInputSchema.safeParse({ title: 'A', sourceWorkspacePath: '~/sites/a' }).success).toBe(false);
    expect(siteServeInputSchema.safeParse({ title: 'A', sourceWorkspacePath: '~/sites/a', mode: 'deploy' }).success).toBe(false);
    expect(siteServeInputSchema.safeParse({ title: 'A', sourceWorkspacePath: '~/sites/a', mode: 'publish', slug: 'a' }).success).toBe(false);
    expect(siteListInputSchema.safeParse({ siteReference: ' site-1 ' }).success).toBe(false);
    expect(siteUnserveInputSchema.safeParse({}).success).toBe(false);
    expect(siteUnserveInputSchema.safeParse({ siteID: 'site-1' }).success).toBe(false);
  });

  test('requires operation-specific site result shapes', () => {
    const previewServeResult = {
      siteID: 'site-1',
      slug: 'customer-support-quarterly',
      mode: SiteServeMode.Preview,
      previewURL: 'https://customer-support-quarterly.example/__preview/preview-1',
      sourceSHA256: '254cc09182b94752e96474af9ba307f74dcfff4e8dfa5b0c4a76f97e634c1c28',
    };
    const publishServeResult = {
      siteID: 'site-1',
      slug: 'customer-support-quarterly',
      mode: SiteServeMode.Publish,
      publishedURL: 'https://customer-support-quarterly.example',
      sourceSHA256: '254cc09182b94752e96474af9ba307f74dcfff4e8dfa5b0c4a76f97e634c1c28',
    };
    const listResult = {
      sites: [{
        siteID: 'site-1',
        slug: 'customer-support-quarterly',
        title: 'customer support quarterly settlement',
        status: SiteLifecycleStatus.Published,
        publishedURL: 'https://customer-support-quarterly.example',
        updatedAt: '2026-07-19T12:00:00Z',
      }],
    };
    const unserveResult = { siteID: 'site-1', slug: 'customer-support-quarterly', unserved: true };

    expect(siteServeResultSchema.safeParse(previewServeResult).success).toBe(true);
    expect(siteServeResultSchema.safeParse(publishServeResult).success).toBe(true);
    expect(siteListResultSchema.safeParse(listResult).success).toBe(true);
    expect(siteListResultSchema.safeParse({ sites: [] }).success).toBe(true);
    expect(siteUnserveResultSchema.safeParse(unserveResult).success).toBe(true);

    expect(siteServeResultSchema.safeParse({ ...publishServeResult, sourceSHA256: undefined }).success).toBe(false);
    expect(siteServeResultSchema.safeParse({ ...publishServeResult, mode: undefined }).success).toBe(false);
    expect(siteServeResultSchema.safeParse({ ...publishServeResult, slug: 'Invalid Slug' }).success).toBe(false);
    expect(siteListResultSchema.safeParse({ sites: [{ siteID: 'site-1' }] }).success).toBe(false);
    expect(siteUnserveResultSchema.safeParse({ siteID: 'site-1', slug: 'a', unserved: false }).success).toBe(false);
    expect(siteServeResultSchema.safeParse({ ...publishServeResult, extra: true }).success).toBe(false);
  });

  test('publishes mode-conditional serve effects and unserve completion evidence', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const serveTool = catalog.tools.find(tool => tool.name === SiteToolName.Serve);
    const listTool = catalog.tools.find(tool => tool.name === SiteToolName.List);
    const unserveTool = catalog.tools.find(tool => tool.name === SiteToolName.Unserve);

    expect(serveTool?.resultContract?.effects).toEqual([
      {
        objectType: 'website',
        effect: 'previewed',
        resultField: 'previewURL',
        effectIdentity: ResourceEffectIdentity.URL,
        when: { resultField: 'mode', equals: 'preview' },
      },
      {
        objectType: 'website',
        effect: 'published',
        resultField: 'publishedURL',
        effectIdentity: ResourceEffectIdentity.URL,
        when: { resultField: 'mode', equals: 'publish' },
      },
    ]);
    expect(serveTool?.requiresApproval).toBeUndefined();
    expect(listTool?.resultContract?.effects).toEqual([]);
    expect(unserveTool?.resultContract?.effects).toEqual([
      { objectType: 'website', effect: 'deleted', resultField: 'siteID', effectIdentity: ResourceEffectIdentity.ID },
    ]);
    expect(unserveTool?.requiresApproval).toBe(true);
    expect(unserveTool?.completionEvidence).toEqual({
      mode: 'success',
      action: 'delete_site',
      targetKind: 'site',
    });
  });

  test('validates document and image read inputs without material aliases', () => {
    expect(documentReadInputSchema.safeParse({
      path: '/workspace/shared/report.pdf',
      maxPages: 10,
      maxOutputBytes: 200000,
    }).success).toBe(true);
    expect(imageReadInputSchema.safeParse({ path: '/workspace/shared/logo.png' }).success).toBe(true);

    expect(documentReadInputSchema.safeParse({ materialID: 'material-1' }).success).toBe(false);
    expect(documentReadInputSchema.safeParse({ path: '/workspace/shared/report.pdf', ocrMode: 'always' }).success).toBe(false);
    expect(documentReadInputSchema.safeParse({ path: '/workspace/shared/report.pdf', maxPages: 0 }).success).toBe(false);
    expect(documentReadInputSchema.safeParse({ path: '/workspace/shared/report.pdf', maxPages: 501 }).success).toBe(false);
    expect(documentReadInputSchema.safeParse({ path: '/workspace/shared/report.pdf', maxOutputBytes: 0 }).success).toBe(false);
    expect(documentReadInputSchema.safeParse({ path: '/workspace/shared/report.pdf', maxOutputBytes: 1 }).success).toBe(false);
    expect(imageReadInputSchema.safeParse({ materialID: 'material-1' }).success).toBe(false);
    expect(imageReadInputSchema.safeParse({ path: '/workspace/shared/logo.png', materialID: 'material-1' }).success).toBe(false);
  });

  test('requires exact document and image read result contracts', () => {
    const documentResult = {
      status: 'ok',
      path: '/workspace/shared/report.pdf',
      format: 'markdown',
      content: '# Report',
      warnings: [],
      truncated: false,
      backend: 'anydoc',
      model: '',
    };
    const imageResult = {
      status: 'ok',
      path: '/workspace/shared/logo.png',
      attachments: [{
        devicePath: '/workspace/shared/logo.png',
        filename: 'logo.png',
        contentType: 'image/png',
        sizeBytes: 3,
        contentBase64: 'YWJj',
      }],
    };

    expect(documentReadResultSchema.safeParse(documentResult).success).toBe(true);
    expect(imageReadResultSchema.safeParse(imageResult).success).toBe(true);
    expect(documentReadResultSchema.safeParse({ ...documentResult, warnings: undefined }).success).toBe(false);
    expect(documentReadResultSchema.safeParse({ ...documentResult, format: 'text' }).success).toBe(false);
    expect(documentReadResultSchema.safeParse({ ...documentResult, extra: true }).success).toBe(false);
    expect(imageReadResultSchema.safeParse({ ...imageResult, attachments: [{ ...imageResult.attachments[0], sizeBytes: -1 }] }).success).toBe(false);
    expect(imageReadResultSchema.safeParse({ ...imageResult, attachments: [{ ...imageResult.attachments[0], devicePath: undefined }] }).success).toBe(false);
    expect(imageReadResultSchema.safeParse({ ...imageResult, extra: true }).success).toBe(false);
  });

  test('publishes mandatory read result contracts without effects', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const documentTool = catalog.tools.find(tool => tool.name === DocumentToolName.Read);
    const imageTool = catalog.tools.find(tool => tool.name === ImageToolName.Read);

    expect(documentTool?.resultContract?.effects).toEqual([]);
    expect(imageTool?.resultContract?.effects).toEqual([]);
    expect(documentTool?.resultContract?.schema.required).toEqual([
      'status', 'path', 'format', 'content', 'warnings', 'truncated',
    ]);
    expect(imageTool?.resultContract?.schema.required).toEqual(['status', 'path', 'attachments']);
    expect(documentTool?.inputSchema.properties).not.toHaveProperty('materialID');
    expect(imageTool?.inputSchema.properties).not.toHaveProperty('materialID');
  });
});

describe('the leave tools that write an entitlement and a return', () => {
  test('ask before a grant is set and not before somebody says they are back', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const grantTool = catalog.tools.find(tool => tool.name === 'leave_grant_set');
    const returnTool = catalog.tools.find(tool => tool.name === 'leave_return_early');

    expect(grantTool?.requiresApproval).toBe(true);
    expect(returnTool?.requiresApproval).toBeUndefined();
    expect(grantTool?.sideEffectClass).toBe(CapabilitySideEffect.WorkspaceWrite);
    expect(returnTool?.sideEffectClass).toBe(CapabilitySideEffect.WorkspaceWrite);
  });

  test('take a person for a grant and nobody for a return', () => {
    expect(leaveGrantSetInputSchema.safeParse({ personHint: '이샘플', days: 18 }).success).toBe(true);
    expect(leaveGrantSetInputSchema.safeParse({ days: 18 }).success).toBe(false);
    expect(leaveReturnEarlyInputSchema.safeParse({}).success).toBe(true);
    expect(leaveReturnEarlyInputSchema.safeParse({ personHint: '이샘플' }).success).toBe(false);
  });
});

describe('catalog descriptions stay messenger-neutral', () => {
  const namedMessengers = ['mattermost', 'buzz'];

  function stringsIn(value: unknown): string[] {
    if (typeof value === 'string') return [value];
    if (Array.isArray(value)) return value.flatMap(stringsIn);
    if (value !== null && typeof value === 'object') {
      return Object.values(value as Record<string, unknown>).flatMap(stringsIn);
    }
    return [];
  }

  test('no tool description names a messenger the way its adapter does', () => {
    const catalog = buildCapabilityToolCatalog(protocolVersion);
    const violations = catalog.tools.flatMap(tool =>
      stringsIn(tool)
        .filter(text => namedMessengers.some(messenger => text.toLowerCase().includes(messenger)))
        .map(text => `${tool.name}: "${text}"`)
    );

    expect(violations).toEqual([]);
  });
});
