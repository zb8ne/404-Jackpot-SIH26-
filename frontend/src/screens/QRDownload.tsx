import { useCallback, useEffect, useState } from 'react'
import { API } from '../api'

export function QRDownload({ documentId }: { documentId: string }) {
  const [error, setError] = useState('')

  const download = useCallback(async () => {
    setError('')
    try {
      const response = await fetch(`${API}/qr/${encodeURIComponent(documentId)}/download.png`)
      if (!response.ok) throw new Error((await response.text()).trim() || 'QR download failed')
      const objectURL = URL.createObjectURL(await response.blob())
      const anchor = document.createElement('a')
      anchor.href = objectURL
      anchor.download = `${documentId}-qr.png`
      anchor.click()
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [documentId])

  useEffect(() => { void download() }, [download])

  return <main className="flex min-h-screen items-center justify-center px-6"><section className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/60 p-8 text-center"><p className="text-xs font-semibold uppercase tracking-widest text-sky-400">Credential QR</p><h1 className="mt-2 text-2xl font-black">QR PNG download</h1><p className="mt-3 font-mono text-sm text-slate-400">{documentId}</p>{error && <p className="mt-5 text-red-300">{error}</p>}<button type="button" onClick={() => void download()} className="mt-6 w-full rounded-xl bg-sky-600 px-5 py-3 font-semibold text-white hover:bg-sky-500">Download QR PNG</button></section></main>
}
