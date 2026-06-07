/**
 * components/ui/JobTaskbar.tsx
 *
 * Persistent bottom taskbar for background jobs.
 *
 * Lives in AppShell so it survives page navigation. When a job is active:
 *   Collapsed (40px): shows label + status pill + progress % + expand / dismiss
 *   Expanded:  drag-resizable panel (min 150px, max 80vh) with xterm log stream
 *
 * Behaviour:
 *   - Auto-expands when a new job starts (activeJobId changes to a new non-null value)
 *   - Expanded height is persisted to localStorage
 *   - Dismissing hides the taskbar but does not cancel the running job
 *   - Drag handle at the top edge of the expanded panel (pointer capture)
 *   - Terminal re-fetches log history on each expand; live WS stream fills the rest
 */

import { useState, useEffect, useRef, useCallback } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { Icon } from './Icon'
import { useJobStore } from '@/stores/jobs'
import { useJob } from '@/hooks/useJob'
import { useWsStore } from '@/stores/ws'
import { api } from '@/lib/api'

// ─── Constants ────────────────────────────────────────────────────────────────

const HEIGHT_KEY  = 'dplane:taskbar:height'
const STRIP_H     = 40   // px - collapsed strip height
const MIN_H       = 150  // px - minimum expanded height
const MAX_H_VH    = 0.80 // fraction of viewport height

function clampHeight(h: number): number {
  return Math.max(MIN_H, Math.min(h, Math.floor(window.innerHeight * MAX_H_VH)))
}

function loadHeight(): number {
  try {
    const saved = localStorage.getItem(HEIGHT_KEY)
    if (saved) return clampHeight(parseInt(saved, 10))
  } catch { /* ignore */ }
  return 280
}

function saveHeight(h: number): void {
  try { localStorage.setItem(HEIGHT_KEY, String(h)) } catch { /* ignore */ }
}

// ─── Terminal pane (xterm, mounts only when expanded) ────────────────────────

function TerminalPane({ jobId, height }: { jobId: string; height: number }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef  = useRef<FitAddon | null>(null)
  const wsOn    = useWsStore((s) => s.on)
  const [ready, setReady] = useState(false)

  // Initialise terminal
  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      theme: {
        background: '#0a0b10',
        foreground: '#e2e8f0',
        cursor:     '#a78bfa',
        selectionBackground: 'rgba(167,139,250,0.3)',
        black: '#1e293b', red: '#f87171', green: '#4ade80',
        yellow: '#fbbf24', blue: '#60a5fa', magenta: '#c084fc',
        cyan: '#22d3ee', white: '#e2e8f0',
      },
      fontFamily: '"JetBrains Mono Variable", monospace',
      fontSize: 12,
      lineHeight: 1.4,
      scrollback: 2000,
      convertEol: true,
      disableStdin: true,
    })

    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(containerRef.current)
    fit.fit()
    termRef.current = term
    fitRef.current  = fit

    // Replay log history from daemon
    api.get<{ logs?: string[] }>(`/api/jobs/${jobId}`)
      .then(data => {
        data.logs?.forEach(line => term.write(line + '\r\n'))
        setReady(true)
      })
      .catch(err => {
        term.write(`\x1b[31m[error fetching log history: ${(err as Error).message}]\x1b[0m\r\n`)
        setReady(true)
      })

    const ro = new ResizeObserver(() => fitRef.current?.fit())
    ro.observe(containerRef.current)

    return () => {
      ro.disconnect()
      term.dispose()
    }
  }, [jobId])

  // Re-fit when panel height changes
  useEffect(() => {
    fitRef.current?.fit()
  }, [height])

  // Stream live log lines
  useEffect(() => {
    if (!ready) return
    return wsOn('jobLog', (msg) => {
      if (msg.job_id === jobId) termRef.current?.write(msg.line + '\r\n')
    })
  }, [ready, jobId, wsOn])

  return (
    <div
      ref={containerRef}
      style={{
        flex: 1,
        background: '#0a0b10',
        padding: 6,
        overflow: 'hidden',
      }}
    />
  )
}

// ─── JobTaskbar ───────────────────────────────────────────────────────────────

interface JobTaskbarProps {
  sidebarCollapsed: boolean
}

export function JobTaskbar({ sidebarCollapsed }: JobTaskbarProps) {
  const { activeJobId, activeJobLabel, setActiveJob } = useJobStore()
  const { data: job } = useJob(activeJobId)

  const [expanded, setExpanded] = useState(false)
  const [height, setHeight] = useState(loadHeight)

  // Track drag state via refs (no re-renders during drag)
  const dragStartYRef = useRef(0)
  const dragStartHRef = useRef(0)
  const isDraggingRef = useRef(false)

  const prevJobIdRef = useRef<string | null>(null)

  // Auto-expand when a new job starts
  useEffect(() => {
    if (activeJobId && activeJobId !== prevJobIdRef.current) {
      setExpanded(true)
    }
    prevJobIdRef.current = activeJobId
  }, [activeJobId])

  // ── Drag handle ──
  const onDragPointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.currentTarget.setPointerCapture(e.pointerId)
    isDraggingRef.current = true
    dragStartYRef.current = e.clientY
    dragStartHRef.current = height
  }, [height])

  const onDragPointerMove = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDraggingRef.current || !e.buttons) return
    const delta = dragStartYRef.current - e.clientY // upward drag = bigger height
    const newH = clampHeight(dragStartHRef.current + delta)
    setHeight(newH)
  }, [])

  const onDragPointerUp = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    isDraggingRef.current = false
    e.currentTarget.releasePointerCapture(e.pointerId)
    saveHeight(clampHeight(dragStartHRef.current + (dragStartYRef.current - e.clientY)))
  }, [])

  if (!activeJobId) return null

  const sidebarLeft = sidebarCollapsed ? 'var(--sidebar-width-collapsed)' : 'var(--sidebar-width)'
  const status = job?.status ?? 'running'
  const progress = job?.progress?.percent

  const statusColor = status === 'done' ? 'var(--success)'
    : status === 'failed'             ? 'var(--error)'
    : 'var(--primary)'

  const statusIcon = status === 'done' ? 'check_circle'
    : status === 'failed'           ? 'error'
    : 'autorenew'

  return (
    <div
      style={{
        position: 'fixed',
        bottom: 0,
        left: sidebarLeft,
        right: 0,
        zIndex: 'var(--z-topbar)' as React.CSSProperties['zIndex'],
        display: 'flex',
        flexDirection: 'column',
        background: 'hsla(var(--hue-bg), 18%, 6%, 0.96)',
        borderTop: '1px solid var(--border)',
        backdropFilter: 'var(--blur-glass)',
        boxShadow: '0 -4px 24px rgba(0,0,0,0.4)',
        transition: 'left 0.2s ease',
      }}
    >
      {/* ── Drag handle (only when expanded) ── */}
      {expanded && (
        <div
          onPointerDown={onDragPointerDown}
          onPointerMove={onDragPointerMove}
          onPointerUp={onDragPointerUp}
          style={{
            height: 6,
            cursor: 'ns-resize',
            background: 'transparent',
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <div style={{
            width: 32, height: 3, borderRadius: 999,
            background: 'var(--border)',
            opacity: 0.5,
          }} />
        </div>
      )}

      {/* ── Terminal pane (only when expanded) ── */}
      {expanded && (
        <div style={{ height, flexShrink: 0, display: 'flex', overflow: 'hidden' }}>
          <TerminalPane jobId={activeJobId} height={height} />
        </div>
      )}

      {/* ── Collapsed strip (always shown) ── */}
      <div
        style={{
          height: STRIP_H,
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 16px',
          userSelect: 'none',
        }}
      >
        {/* Status icon */}
        <Icon
          name={statusIcon}
          size={16}
          style={{
            color: statusColor,
            flexShrink: 0,
            animation: status === 'running' ? 'spin 2s linear infinite' : 'none',
          }}
        />

        {/* Label + progress */}
        <div style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{
            fontSize: 'var(--text-xs)', fontWeight: 600,
            color: 'var(--text-secondary)',
            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }}>
            {activeJobLabel || 'Background task'}
          </span>

          {/* Status pill */}
          <span style={{
            padding: '1px 7px', borderRadius: 99, flexShrink: 0,
            fontSize: 'var(--text-2xs)', fontWeight: 700, textTransform: 'uppercase',
            letterSpacing: '0.4px',
            background: status === 'done'   ? 'var(--success-bg)'
              : status === 'failed'         ? 'var(--error-bg)'
              : 'hsla(var(--hue-primary),100%,72%,.08)',
            border: `1px solid ${status === 'done' ? 'var(--success-border)'
              : status === 'failed'                ? 'var(--error-border)'
              : 'hsla(var(--hue-primary),100%,72%,.2)'}`,
            color: statusColor,
          }}>
            {progress !== undefined && status === 'running' ? `${progress}%` : status}
          </span>
        </div>

        {/* Progress bar (running only) */}
        {status === 'running' && progress !== undefined && (
          <div style={{
            width: 80, height: 3, flexShrink: 0,
            background: 'var(--border)', borderRadius: 999, overflow: 'hidden',
          }}>
            <div style={{
              height: '100%', width: `${progress}%`,
              background: 'var(--primary)', borderRadius: 999,
              transition: 'width 0.4s ease',
            }} />
          </div>
        )}

        {/* Expand / collapse toggle */}
        <button
          onClick={() => setExpanded(e => !e)}
          aria-label={expanded ? 'Collapse job console' : 'Expand job console'}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            width: 26, height: 26, flexShrink: 0, borderRadius: 'var(--radius-sm)',
            background: 'none', border: '1px solid var(--border)',
            cursor: 'pointer', color: 'var(--text-tertiary)',
            transition: 'all 0.15s',
          }}
          onMouseEnter={e => { e.currentTarget.style.background = 'rgba(255,255,255,0.06)'; e.currentTarget.style.color = 'var(--text)' }}
          onMouseLeave={e => { e.currentTarget.style.background = 'none'; e.currentTarget.style.color = 'var(--text-tertiary)' }}
        >
          <Icon name={expanded ? 'expand_more' : 'expand_less'} size={14} />
        </button>

        {/* Dismiss */}
        <button
          onClick={() => { setExpanded(false); setActiveJob(null) }}
          aria-label="Dismiss job taskbar"
          title="Hide job taskbar (job continues in background)"
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            width: 26, height: 26, flexShrink: 0, borderRadius: 'var(--radius-sm)',
            background: 'none', border: '1px solid var(--border)',
            cursor: 'pointer', color: 'var(--text-tertiary)',
            transition: 'all 0.15s',
          }}
          onMouseEnter={e => { e.currentTarget.style.background = 'rgba(255,255,255,0.06)'; e.currentTarget.style.color = 'var(--text)' }}
          onMouseLeave={e => { e.currentTarget.style.background = 'none'; e.currentTarget.style.color = 'var(--text-tertiary)' }}
        >
          <Icon name="close" size={13} />
        </button>
      </div>
    </div>
  )
}
