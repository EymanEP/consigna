import { describe, expect, it } from 'vitest';

import { classifyError } from './uploads';

describe('classifyError', () => {
  it.each([0, 404, 408, 409, 423, 429, 500, 502, 503])('pauses and retries on %d', (status) => {
    expect(classifyError(status).kind).toBe('pause');
  });

  it.each([400, 401, 403, 412, 413, 415])('fails on %d', (status) => {
    expect(classifyError(status).kind).toBe('fail');
  });

  it('explains a full tray', () => {
    const action = classifyError(413);
    expect(action.kind === 'fail' && action.message).toMatch(/space/i);
  });
});
