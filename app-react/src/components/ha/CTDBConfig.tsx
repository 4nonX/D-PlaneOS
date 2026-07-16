/**
 * components/ha/CTDBConfig.tsx - CTDB clustering configuration UI
 *
 * Enables Samba clustering (CTDB) for SMB HA failover without client disconnects.
 * Requires: HA enabled, Samba enabled
 *
 * API: GET/POST /api/ha/ctdb/configure
 *      GET /api/ha/ctdb/status
 *      GET /api/ha/ctdb/databases
 */

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { Icon } from '@/components/ui/Icon'
import { toast } from '@/hooks/useToast'
import { Skeleton } from '@/components/ui/LoadingSpinner'

interface CTDBConfig {
  enable:           boolean
  data_pool:        string
  data_dataset:     string
  public_addresses: string[]
  node_timeout:     number
  recovery_timeout: number
  log_level:        number
}

interface CTDBNodeStatus {
  node_id:     number
  address:     string
  status:      string
  role:        string
  generation:  number
}

interface CTDBStatusResponse {
  success:        boolean
  enabled:        boolean
  daemon_running: boolean
  cluster_status: string
  nodes:          CTDBNodeStatus[]
  public_ips:     Record<string, string>
  error?:         string
}

export function CTDBConfig() {
  const queryClient = useQueryClient()
  const [formData, setFormData] = useState<CTDBConfig | null>(null)
  const [newAddress, setNewAddress] = useState('')

  // Fetch current config
  const { data: configData, isLoading: configLoading } = useQuery({
    queryKey: ['ha', 'ctdb', 'config'],
    queryFn: async () => {
      const res = await api.get<{ success: boolean; config: CTDBConfig }>('/api/ha/ctdb/configure')
      return res.config
    },
  })

  // Fetch cluster status
  const { data: statusData, isLoading: statusLoading } = useQuery({
    queryKey: ['ha', 'ctdb', 'status'],
    queryFn: async () => {
      const res = await api.get<CTDBStatusResponse>('/api/ha/ctdb/status')
      return res
    },
  })

  // Save config mutation
  const saveMutation = useMutation({
    mutationFn: async (config: CTDBConfig) => {
      return await api.post('/api/ha/ctdb/configure', config)
    },
    onSuccess: () => {
      toast.success('CTDB configuration saved')
      queryClient.invalidateQueries({ queryKey: ['ha', 'ctdb'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Failed to save CTDB configuration')
    },
  })

  // Initialize form when config loads
  if (configData && !formData) {
    setFormData(configData)
  }

  if (configLoading || !formData) {
    return <Skeleton height={400} />
  }

  const handleSave = () => {
    if (formData) {
      saveMutation.mutate(formData)
    }
  }

  const handleAddAddress = () => {
    if (newAddress && formData) {
      const updated = {
        ...formData,
        public_addresses: [...formData.public_addresses, newAddress],
      }
      setFormData(updated)
      setNewAddress('')
    }
  }

  const handleRemoveAddress = (idx: number) => {
    if (formData) {
      const updated = {
        ...formData,
        public_addresses: formData.public_addresses.filter((_, i) => i !== idx),
      }
      setFormData(updated)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      {/* Enable/Disable toggle */}
      <section style={{ padding: 16, border: '1px solid var(--border)', borderRadius: 'var(--radius-md)' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
          <div>
            <h3 style={{ margin: '0 0 4px 0', fontSize: '1rem', fontWeight: 600 }}>CTDB Clustering</h3>
            <p style={{ margin: 0, fontSize: '0.875rem', color: 'var(--text-tertiary)' }}>
              Enable Samba clustering (CTDB) for SMB client failover without disconnects
            </p>
          </div>
          <input
            type="checkbox"
            checked={formData.enable}
            onChange={(e) => setFormData({ ...formData, enable: e.target.checked })}
            style={{ width: 24, height: 24 }}
          />
        </div>

        {formData.enable && (
          <div style={{ padding: 12, background: 'var(--primary-bg)', borderRadius: 'var(--radius-sm)', marginBottom: 16 }}>
            <div style={{ display: 'flex', gap: 8 }}>
              <Icon name="info" size={16} style={{ color: 'var(--primary)', flexShrink: 0 }} />
              <div style={{ fontSize: '0.875rem', color: 'var(--primary)' }}>
                CTDB requires: HA enabled, Samba enabled, ZFS dataset {formData.data_dataset} must exist
              </div>
            </div>
          </div>
        )}
      </section>

      {/* Configuration form (visible when enabled) */}
      {formData.enable && (
        <section style={{ padding: 16, border: '1px solid var(--border)', borderRadius: 'var(--radius-md)' }}>
          <h4 style={{ margin: '0 0 16px 0' }}>Configuration</h4>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
            <div>
              <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>Data Pool</label>
              <input
                type="text"
                value={formData.data_pool}
                onChange={(e) => setFormData({ ...formData, data_pool: e.target.value })}
                placeholder="tank"
                style={{ width: '100%', padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>Data Dataset</label>
              <input
                type="text"
                value={formData.data_dataset}
                onChange={(e) => setFormData({ ...formData, data_dataset: e.target.value })}
                placeholder="tank/ctdb"
                style={{ width: '100%', padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}
              />
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
            <div>
              <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>Node Timeout (seconds)</label>
              <input
                type="number"
                min="10"
                value={formData.node_timeout}
                onChange={(e) => setFormData({ ...formData, node_timeout: parseInt(e.target.value) })}
                style={{ width: '100%', padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>Recovery Timeout (seconds)</label>
              <input
                type="number"
                min="30"
                value={formData.recovery_timeout}
                onChange={(e) => setFormData({ ...formData, recovery_timeout: parseInt(e.target.value) })}
                style={{ width: '100%', padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}
              />
            </div>
          </div>

          <div>
            <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>Log Level</label>
            <select
              value={formData.log_level}
              onChange={(e) => setFormData({ ...formData, log_level: parseInt(e.target.value) })}
              style={{ width: '100%', padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}
            >
              <option value="0">0 - ERROR</option>
              <option value="1">1 - WARNING</option>
              <option value="2">2 - NOTICE</option>
              <option value="3">3 - INFO</option>
              <option value="4">4 - DEBUG</option>
            </select>
          </div>
        </section>
      )}

      {/* Public Addresses */}
      {formData.enable && (
        <section style={{ padding: 16, border: '1px solid var(--border)', borderRadius: 'var(--radius-md)' }}>
          <h4 style={{ margin: '0 0 16px 0' }}>Public IP Addresses (VIPs)</h4>
          <p style={{ margin: '0 0 12px 0', fontSize: '0.875rem', color: 'var(--text-tertiary)' }}>
            CTDB will manage these IPs across cluster nodes. Format: 192.168.1.100/24 eth0
          </p>

          {formData.public_addresses.length > 0 && (
            <div style={{ marginBottom: 12 }}>
              {formData.public_addresses.map((addr, idx) => (
                <div
                  key={idx}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '8px 10px',
                    marginBottom: 8,
                    background: 'var(--surface)',
                    border: '1px solid var(--border)',
                    borderRadius: 'var(--radius-sm)',
                  }}
                >
                  <span>{addr}</span>
                  <button
                    onClick={() => handleRemoveAddress(idx)}
                    style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--danger)' }}
                  >
                    <Icon name="delete" size={18} />
                  </button>
                </div>
              ))}
            </div>
          )}

          <div style={{ display: 'flex', gap: 8 }}>
            <input
              type="text"
              value={newAddress}
              onChange={(e) => setNewAddress(e.target.value)}
              placeholder="192.168.1.100/24 eth0"
              style={{ flex: 1, padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)' }}
            />
            <button
              onClick={handleAddAddress}
              style={{
                padding: '8px 16px',
                background: 'var(--primary)',
                color: 'white',
                border: 'none',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
              }}
            >
              Add
            </button>
          </div>
        </section>
      )}

      {/* Cluster Status */}
      {statusData?.enabled && (
        <section style={{ padding: 16, border: '1px solid var(--border)', borderRadius: 'var(--radius-md)' }}>
          <h4 style={{ margin: '0 0 16px 0' }}>CTDB Cluster Status</h4>

          {statusLoading ? (
            <Skeleton height={150} />
          ) : statusData ? (
            <>
              <div style={{ marginBottom: 16 }}>
                <span style={{ fontWeight: 500 }}>Status: </span>
                <span
                  style={{
                    padding: '2px 8px',
                    borderRadius: 'var(--radius-sm)',
                    background:
                      statusData.cluster_status === 'healthy'
                        ? 'var(--success-bg)'
                        : statusData.cluster_status === 'degraded'
                          ? 'var(--warning-bg)'
                          : 'var(--danger-bg)',
                    color:
                      statusData.cluster_status === 'healthy'
                        ? 'var(--success)'
                        : statusData.cluster_status === 'degraded'
                          ? 'var(--warning)'
                          : 'var(--danger)',
                  }}
                >
                  {statusData.cluster_status.toUpperCase()}
                </span>
              </div>

              {statusData.nodes && statusData.nodes.length > 0 && (
                <div>
                  <h5 style={{ margin: '0 0 8px 0', fontSize: '0.875rem' }}>Nodes</h5>
                  {statusData.nodes.map((node) => (
                    <div
                      key={node.node_id}
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        padding: '8px 10px',
                        marginBottom: 8,
                        background: 'var(--surface)',
                        borderRadius: 'var(--radius-sm)',
                      }}
                    >
                      <span>
                        Node {node.node_id} ({node.address})
                      </span>
                      <span
                        style={{
                          fontSize: '0.875rem',
                          color: node.status === 'connected' ? 'var(--success)' : 'var(--danger)',
                        }}
                      >
                        {node.status.toUpperCase()}
                      </span>
                    </div>
                  ))}
                </div>
              )}

              {statusData.public_ips && Object.keys(statusData.public_ips).length > 0 && (
                <div style={{ marginTop: 16 }}>
                  <h5 style={{ margin: '0 0 8px 0', fontSize: '0.875rem' }}>Public IP Ownership</h5>
                  {Object.entries(statusData.public_ips).map(([ip, owner]) => (
                    <div key={ip} style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
                      <span>{ip}</span>
                      <span style={{ fontSize: '0.875rem', color: 'var(--text-tertiary)' }}>{owner}</span>
                    </div>
                  ))}
                </div>
              )}
            </>
          ) : null}
        </section>
      )}

      {/* Save button */}
      {formData.enable && (
        <button
          onClick={handleSave}
          disabled={saveMutation.isPending}
          style={{
            padding: '10px 20px',
            background: 'var(--primary)',
            color: 'white',
            border: 'none',
            borderRadius: 'var(--radius-sm)',
            cursor: 'pointer',
            fontWeight: 500,
          }}
        >
          {saveMutation.isPending ? 'Saving...' : 'Save CTDB Configuration'}
        </button>
      )}
    </div>
  )
}
