import { useEffect, useState } from 'react'
import { API } from './api'
import { Citizen } from './screens/Citizen'
import { Issue } from './screens/Issue'
import { Verify } from './screens/Verify'

const TABS = [
  { id: 'verify', label: 'Verify', hint: 'check a document' },
  { id: 'issue', label: 'Issue', hint: 'anchor a new one' },
  { id: 'citizen', label: 'Citizen', hint: "what someone holds" },
] as const

type Tab = (typeof TABS)[number]['id']

export default function App() {
  const [tab, setTab] = useState<Tab>('verify')
  const [contract, setContract] = useState('')

  useEffect(() => {
    fetch(`${API}/health`)
      .then((r) => r.json())
      .then((h) => setContract(h.contract))
      .catch(() => setContract(''))
  }, [])

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-800 bg-slate-900/60">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-6 py-5">
          <div>
            <h1 className="text-2xl font-black tracking-tight text-slate-100">
              Government Credential Registry
            </h1>
            <p className="text-sm text-slate-500">
              {contract ? (
                <>
                  contract <span className="font-mono">{contract}</span>
                </>
              ) : (
                'backend unreachable — is `make demo` running?'
              )}
            </p>
          </div>

          <nav className="flex gap-2">
            {TABS.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`rounded-xl px-5 py-3 text-left transition ${
                  tab === t.id
                    ? 'bg-sky-600 text-white'
                    : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'
                }`}
              >
                <div className="font-semibold">{t.label}</div>
                <div className="text-xs opacity-70">{t.hint}</div>
              </button>
            ))}
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-10">
        {tab === 'verify' && <Verify />}
        {tab === 'issue' && <Issue />}
        {tab === 'citizen' && <Citizen />}
      </main>
    </div>
  )
}
