import { useState } from 'react'
import { toast } from '@/hooks/useToast'

function getCEUrl(): string {
  const port = window.location.hostname === 'localhost' ? '9001' : '9001'
  const protocol = window.location.protocol
  return `${protocol}//${window.location.hostname}:${port}`
}

export function CompliancePage() {
  const [ceUrl] = useState(getCEUrl())
  const [error, setError] = useState<string>('')

  if (error) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', flexDirection: 'column', gap: 16 }}>
        <div style={{ color: 'var(--error)' }}>Failed to load Compliance Engine</div>
        <div style={{ fontSize: 'var(--text-sm)', color: 'var(--text-secondary)' }}>{error}</div>
        <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
          Ensure the Compliance Engine sidecar is running on port 9001
        </div>
      </div>
    )
  }

  return (
    <iframe
      src={ceUrl}
      style={{ width: '100%', height: '100vh', border: 'none' }}
      title="Compliance Engine"
      onError={() => {
        setError('Could not connect to Compliance Engine at ' + ceUrl)
        toast.error('Failed to load Compliance Engine')
      }}
    />
  )
}
