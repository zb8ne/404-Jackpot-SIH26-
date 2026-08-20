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

  return (
    <div className="min-h-screen bg-slate-950">
      <header className="border-b border-slate-800 bg-slate-900/80 backdrop-blur">
        <div className="mx-auto max-w-[1600px] px-6 py-5">
          <div className="flex flex-wrap items-start justify-between gap-5">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.24em] text-sky-400">Secure credential registry</p>
              <h1 className="mt-2 text-2xl font-black tracking-tight text-slate-100">
                {government ? 'Government workspace' : 'Citizen portal'}
              </h1>
              <p className="mt-1 text-sm text-slate-500">
                {contract ? <>Registry <span className="font-mono">{shorten(contract)}</span></> : 'Backend unavailable'}
              </p>
            </div>

            <div className="flex items-center gap-4 rounded-xl border border-slate-800 bg-slate-950/60 px-4 py-3">
              <div className="text-right">
                <p className="font-semibold text-slate-100">{displayName}</p>
                <p className="text-xs text-slate-400">{roleLabel} · {contextLabel}</p>
              </div>
              <button type="button" onClick={onSignOut} className="rounded-lg border border-slate-700 px-3 py-2 text-sm text-slate-300 hover:bg-slate-800">
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
