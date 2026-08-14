import type { Verdict } from './api'

// One place deciding what each verdict looks like, so the verify panel and the
// citizen badges can never drift apart.
export const VERDICTS: Record<
  Verdict,
  { label: string; headline: string; panel: string; badge: string; accent: string; icon: string }
> = {
  VALID: {
    label: 'Valid',
    headline: 'Authentic and current',
    panel: 'border-emerald-500/60 bg-emerald-500/10',
    badge: 'border-emerald-500/50 bg-emerald-500/15 text-emerald-300',
    accent: 'text-emerald-400',
    icon: '✓',
  },
  SUPERSEDED: {
    label: 'Superseded',
    headline: 'Genuine, but replaced by a newer version',
    panel: 'border-amber-500/60 bg-amber-500/10',
    badge: 'border-amber-500/50 bg-amber-500/15 text-amber-300',
    accent: 'text-amber-400',
    icon: '↻',
  },
  REVOKED: {
    label: 'Revoked',
    headline: 'Genuine, but revoked by the issuing department',
    panel: 'border-red-500/60 bg-red-500/10',
    badge: 'border-red-500/50 bg-red-500/15 text-red-300',
    accent: 'text-red-400',
    icon: '⊘',
  },
  TAMPERED: {
    label: 'Tampered',
    headline: 'This file has been modified since it was issued',
    panel: 'border-red-500/60 bg-red-500/10',
    badge: 'border-red-500/50 bg-red-500/15 text-red-300',
    accent: 'text-red-400',
    icon: '✕',
  },
  NOT_ISSUED: {
    label: 'Not issued',
    headline: 'No such document was ever issued',
    panel: 'border-slate-600 bg-slate-800/40',
    badge: 'border-slate-600 bg-slate-700/40 text-slate-300',
    accent: 'text-slate-400',
    icon: '?',
  },
  UNKNOWN: {
    label: 'Unknown',
    headline: 'The registry returned a status this build does not recognise',
    panel: 'border-slate-600 bg-slate-800/40',
    badge: 'border-slate-600 bg-slate-700/40 text-slate-300',
    accent: 'text-slate-400',
    icon: '?',
  },
}

export const verdictOf = (status?: string) => VERDICTS[(status as Verdict) ?? 'UNKNOWN'] ?? VERDICTS.UNKNOWN
