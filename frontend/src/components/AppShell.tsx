import type { ReactNode } from 'react'
import type { AuthenticatedAccount } from '../api'

export type ShellTab =
  | 'verify'
  | 'issue'
  | 'revoke'
  | 'supersede'
  | 'citizen'
  | 'audit'
  | 'monitoring'
  | 'my-credentials'
  | 'inbox'
  | 'requests'

type NavigationItem = { id: ShellTab; label: string; hint: string }

const CITIZEN_NAVIGATION: NavigationItem[] = [
  { id: 'my-credentials', label: 'My credentials', hint: 'Issued documents' },
  { id: 'inbox', label: 'Inbox', hint: 'Access requests' },
]

export function AppShell({
  account,
  contract,
  activeTab,
  onTabChange,
  onSignOut,
  children,
}: {
  account: AuthenticatedAccount
  contract: string
  activeTab: ShellTab
  onTabChange: (tab: ShellTab) => void
  onSignOut: () => void
  children: ReactNode
}) {
  const government = account.accountType === 'GOVERNMENT' ? account.governmentProfile : null
  const navigation = government ? [] : CITIZEN_NAVIGATION
  const displayName = government?.name ?? account.citizenProfile?.displayName
  const contextLabel = government
    ? government.department?.name ?? 'System oversight'
    : 'Citizen portal'
  const roleLabel = government ? government.role : 'CITIZEN'
  const initials = (displayName ?? 'User').split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase()
  const accent = government?.role === 'ADMIN'
    ? 'border-[#f07868]/40 bg-[#f07868]/15 text-[#ff9a89]'
    : government?.role === 'OFFICIAL'
      ? 'border-[#65c8a3]/40 bg-[#65c8a3]/15 text-[#82dfbc]'
      : government?.role === 'CONTROLLER'
        ? 'border-[#e8b45b]/40 bg-[#e8b45b]/15 text-[#f2ca7f]'
        : 'border-[#a992e8]/40 bg-[#a992e8]/15 text-[#c2b1f5]'

  return (
    <div className="civic-theme civic-workspace min-h-screen">
      <header className="sticky top-0 z-40 border-b border-[#443b36] bg-[#121617] backdrop-blur-xl">
        <div className="mx-auto max-w-[1600px] px-4 py-3 sm:px-6">
          <div className="flex items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border text-lg font-black ${accent}`}>PS</div>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h1 className="truncate text-sm font-bold text-slate-100">PramanSetu</h1>
                  {government?.department && <span className={`hidden rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider sm:inline ${accent}`}>{government.department.name}</span>}
                </div>
                <div className="mt-0.5 flex items-center gap-2 text-[11px] text-slate-500">
                  <span className={`h-1.5 w-1.5 rounded-full ${contract ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,.8)]' : 'bg-rose-400'}`} />
                  <span>{contract ? 'Registry online' : 'Backend unavailable'}</span>
                  {contract && <span className="hidden font-mono sm:inline">· {shorten(contract)}</span>}
                </div>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <div className="hidden text-right sm:block">
                <p className="text-sm font-semibold text-slate-100">{displayName}</p>
                <p className="text-[11px] uppercase tracking-wider text-slate-500">{roleLabel} · {contextLabel}</p>
              </div>
              <div className="flex h-9 w-9 items-center justify-center rounded-full border border-[#5b4d45] bg-[#292522] text-xs font-black text-[#f4efe7]">{initials}</div>
              <button type="button" onClick={onSignOut} className="rounded-lg border border-slate-700 px-3 py-2 text-xs font-semibold text-slate-400 transition hover:border-slate-500 hover:bg-slate-800 hover:text-slate-100">
                Sign out
              </button>
            </div>
          </div>

          {navigation.length > 0 && <nav aria-label="Workspace navigation" className="mt-6 flex gap-2 overflow-x-auto pb-1">
            {navigation.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => onTabChange(item.id)}
                aria-current={activeTab === item.id ? 'page' : undefined}
                className={`shrink-0 rounded-xl px-4 py-3 text-left transition ${
                  activeTab === item.id
                    ? 'bg-sky-600 text-white shadow-lg shadow-sky-950/30'
                    : 'border border-slate-800 text-slate-400 hover:border-slate-700 hover:bg-slate-800 hover:text-slate-200'
                }`}
              >
                <span className="block text-sm font-semibold">{item.label}</span>
                <span className="block text-xs opacity-70">{item.hint}</span>
              </button>
            ))}
          </nav>}
        </div>
      </header>

      <main className="mx-auto max-w-[1600px] px-4 py-8 sm:px-6">{children}</main>
    </div>
  )
}

function shorten(value: string) {
  return value.length > 18 ? `${value.slice(0, 10)}…${value.slice(-6)}` : value
}
