import { GrafanaPageRef } from '../types';

/**
 * Generate a display label for a tab
 */
export function getTabLabel(ref: GrafanaPageRef, index: number): string {
  if (ref.title) {
    return ref.title.length > 20 ? ref.title.substring(0, 20) + '...' : ref.title;
  }
  if (ref.type === 'dashboard' && ref.uid) {
    return `Dashboard ${ref.uid.substring(0, 8)}`;
  }
  return ref.type === 'explore' ? 'Explore' : `Page ${index + 1}`;
}

/**
 * Convert an absolute URL to a relative URL suitable for iframe embedding.
 * Optionally adds kiosk mode parameter.
 *
 * Returns an empty string for anything that is not a Grafana dashboard or
 * Explore URL: persisted/imported/shared session data is untrusted, and
 * without this guard an attacker-controlled value (external origin, other
 * schemes, or path traversal escaping into other same-origin routes) would
 * end up as the iframe src.
 */
export function toRelativeUrl(url: string, kioskModeEnabled = true): string {
  let relativeUrl = url;

  if (url.startsWith('http://') || url.startsWith('https://')) {
    const match = url.match(/https?:\/\/[^/]+(\/.*)/);
    if (!match) {
      return '';
    }
    relativeUrl = match[1];
  }

  if (!/^\/(d\/|explore)/.test(relativeUrl)) {
    return '';
  }

  // Reject path traversal so "/d/../admin" (or an encoded/backslash variant —
  // browsers treat "\" as "/" in HTTP URLs) cannot escape into another
  // same-origin route inside the iframe.
  const pathOnly = relativeUrl.split('?')[0];
  let decodedPath = pathOnly;
  try {
    decodedPath = decodeURIComponent(pathOnly);
  } catch {
    return '';
  }
  if (pathOnly.includes('\\') || decodedPath.includes('\\')) {
    return '';
  }
  if (/(^|\/)\.\.?(\/|$)/.test(decodedPath) || /%2e/i.test(pathOnly)) {
    return '';
  }

  if (relativeUrl.includes('kiosk') || relativeUrl.includes('viewPanel')) {
    return relativeUrl;
  }

  if (!kioskModeEnabled) {
    return relativeUrl;
  }

  const separator = relativeUrl.includes('?') ? '&' : '?';
  return `${relativeUrl}${separator}kiosk`;
}

/**
 * Extract the path from an absolute URL
 */
export function extractPathFromUrl(url: string): string {
  if (url.startsWith('http://') || url.startsWith('https://')) {
    const match = url.match(/https?:\/\/[^/]+(\/.*)/);
    return match ? match[1] : url;
  }
  return url;
}
