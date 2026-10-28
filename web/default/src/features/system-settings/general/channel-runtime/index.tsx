import { RefreshCw, Unlock } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'

import { SettingsSwitchField } from '../../components/settings-form-layout'
import { SettingsPageActionsPortal } from '../../components/settings-page-context'
import { SettingsSection } from '../../components/settings-section'
import { useUpdateOption } from '../../hooks/use-update-option'
import {
  getChannelRuntimeLimits,
  getChannelRuntimeStatus,
  refreshChannelRuntime,
  unlockChannelRuntime,
} from './api'
import type {
  ChannelRuntimeLimitItem,
  ChannelRuntimeSettings,
  ChannelRuntimeStatusItem,
} from './types'

const RULES_PLACEHOLDER = `[
  {
    "name": "glm5 long context split",
    "model_regex": ["^GLM5.*$"],
    "enabled": true,
    "segments": [
      { "min_length": 0, "max_length": 10000, "channel_ids": [1] },
      { "min_length": 10000, "max_length": 0, "channel_ids": [2, 3, 4] }
    ]
  }
]`

const OVERRIDES_PLACEHOLDER = `[
  { "channel_name": "GLM5-H20-01", "max_batch": 32 },
  { "channel_name": "GLM5-H20-02", "max_batch": 16 }
]`

function formatJsonEditor(value: string, fallback: string): string {
  const raw = (value ?? '').toString().trim()
  if (!raw) return fallback
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return fallback
  }
}

function normalizeJson(value: string | undefined, fallback: string): string {
  try {
    return JSON.stringify(JSON.parse(value || fallback))
  } catch {
    return fallback
  }
}

function formatTimestamp(ts: number): string {
  if (!ts) return '-'
  const date = new Date(ts * 1000)
  return date.toLocaleTimeString()
}

interface Props {
  defaultValues: ChannelRuntimeSettings
}

export function ChannelRuntimeSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const [enabled, setEnabled] = useState(
    props.defaultValues['channel_runtime_setting.enabled']
  )
  const [scrapeInterval, setScrapeInterval] = useState(
    props.defaultValues['channel_runtime_setting.scrape_interval_seconds']
  )
  const [scrapeTimeout, setScrapeTimeout] = useState(
    props.defaultValues['channel_runtime_setting.scrape_timeout_ms']
  )
  const [retentionHours, setRetentionHours] = useState(
    props.defaultValues['channel_runtime_setting.sample_retention_hours']
  )
  const [defaultMaxBatch, setDefaultMaxBatch] = useState(
    props.defaultValues['channel_runtime_setting.default_max_batch']
  )
  const [scrapeOnlyChannels, setScrapeOnlyChannels] = useState(
    props.defaultValues['channel_runtime_setting.scrape_only_channels'] || '[]'
  )
  const [maxBatchOverrides, setMaxBatchOverrides] = useState(
    props.defaultValues['channel_runtime_setting.max_batch_overrides'] || '[]'
  )
  const [lockEnabled, setLockEnabled] = useState(
    props.defaultValues['channel_runtime_setting.saturation_lock_enabled']
  )
  const [waitingSamples, setWaitingSamples] = useState(
    props.defaultValues['channel_runtime_setting.saturation_waiting_samples']
  )
  const [releaseSamples, setReleaseSamples] = useState(
    props.defaultValues['channel_runtime_setting.saturation_release_samples']
  )
  const [maxLockSeconds, setMaxLockSeconds] = useState(
    props.defaultValues['channel_runtime_setting.saturation_max_lock_seconds']
  )
  const [lengthRoutingEnabled, setLengthRoutingEnabled] = useState(
    props.defaultValues['channel_runtime_setting.length_routing_enabled']
  )
  const [lengthFallback, setLengthFallback] = useState(
    props.defaultValues['channel_runtime_setting.length_routing_fallback']
  )
  const [lengthRules, setLengthRules] = useState(
    formatJsonEditor(
      props.defaultValues['channel_runtime_setting.length_routing_rules'],
      '[]'
    )
  )

  const [statusItems, setStatusItems] = useState<ChannelRuntimeStatusItem[]>([])
  const [limits, setLimits] = useState<ChannelRuntimeLimitItem[]>([])
  const [loadingStatus, setLoadingStatus] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const values = props.defaultValues
    setEnabled(values['channel_runtime_setting.enabled'])
    setScrapeInterval(
      values['channel_runtime_setting.scrape_interval_seconds']
    )
    setScrapeTimeout(values['channel_runtime_setting.scrape_timeout_ms'])
    setRetentionHours(values['channel_runtime_setting.sample_retention_hours'])
    setDefaultMaxBatch(values['channel_runtime_setting.default_max_batch'])
    setScrapeOnlyChannels(
      values['channel_runtime_setting.scrape_only_channels'] || '[]'
    )
    setMaxBatchOverrides(
      values['channel_runtime_setting.max_batch_overrides'] || '[]'
    )
    setLockEnabled(values['channel_runtime_setting.saturation_lock_enabled'])
    setWaitingSamples(
      values['channel_runtime_setting.saturation_waiting_samples']
    )
    setReleaseSamples(
      values['channel_runtime_setting.saturation_release_samples']
    )
    setMaxLockSeconds(
      values['channel_runtime_setting.saturation_max_lock_seconds']
    )
    setLengthRoutingEnabled(
      values['channel_runtime_setting.length_routing_enabled']
    )
    setLengthFallback(
      values['channel_runtime_setting.length_routing_fallback']
    )
    setLengthRules(
      formatJsonEditor(
        values['channel_runtime_setting.length_routing_rules'],
        '[]'
      )
    )
  }, [props.defaultValues])

  const refreshStatus = useCallback(async () => {
    setLoadingStatus(true)
    try {
      const [statusRes, limitsRes] = await Promise.all([
        getChannelRuntimeStatus(),
        getChannelRuntimeLimits(),
      ])
      if (statusRes.success) setStatusItems(statusRes.data?.items || [])
      if (limitsRes.success) setLimits(limitsRes.data || [])
    } catch {
      toast.error(t('Failed to load runtime status'))
    } finally {
      setLoadingStatus(false)
    }
  }, [t])

  useEffect(() => {
    refreshStatus()
  }, [refreshStatus])

  const handleRefresh = async () => {
    const res = await refreshChannelRuntime()
    if (res.success) {
      toast.success(t('Collection triggered'))
      // 采集是异步的，稍等再拉一次结果
      setTimeout(refreshStatus, 2000)
    } else {
      toast.error(res.message || t('Failed to trigger collection'))
    }
  }

  const handleUnlock = async (channelId: number) => {
    const res = await unlockChannelRuntime(channelId)
    if (res.success) {
      toast.success(t('Unlocked'))
      refreshStatus()
    } else {
      toast.error(res.message || t('Failed to unlock'))
    }
  }

  const validateJson = (value: string, label: string): string | null => {
    try {
      const parsed = JSON.parse(value || '[]')
      return JSON.stringify(parsed)
    } catch {
      toast.error(`${t('Invalid JSON')}: ${label}`)
      return null
    }
  }

  const handleSave = async () => {
    const overridesJson = validateJson(maxBatchOverrides, 'max_batch_overrides')
    if (overridesJson === null) return
    const rulesJson = validateJson(lengthRules, 'length_routing_rules')
    if (rulesJson === null) return
    const scrapeOnlyJson = validateJson(
      scrapeOnlyChannels,
      'scrape_only_channels'
    )
    if (scrapeOnlyJson === null) return
    const origin = props.defaultValues
    const updates: { key: string; value: string }[] = []
    const push = (key: string, value: string | number | boolean) => {
      updates.push({ key, value: String(value) })
    }

    const pushIfChanged = (
      key: keyof ChannelRuntimeSettings,
      value: string | number | boolean
    ) => {
      if (String(value) !== String(origin[key])) {
        push(String(key), value)
      }
    }

    pushIfChanged('channel_runtime_setting.enabled', enabled)
    pushIfChanged(
      'channel_runtime_setting.scrape_interval_seconds',
      scrapeInterval
    )
    pushIfChanged('channel_runtime_setting.scrape_timeout_ms', scrapeTimeout)
    pushIfChanged(
      'channel_runtime_setting.sample_retention_hours',
      retentionHours
    )
    pushIfChanged(
      'channel_runtime_setting.default_max_batch',
      defaultMaxBatch
    )
    pushIfChanged(
      'channel_runtime_setting.saturation_lock_enabled',
      lockEnabled
    )
    pushIfChanged(
      'channel_runtime_setting.saturation_waiting_samples',
      waitingSamples
    )
    pushIfChanged(
      'channel_runtime_setting.saturation_release_samples',
      releaseSamples
    )
    pushIfChanged(
      'channel_runtime_setting.saturation_max_lock_seconds',
      maxLockSeconds
    )
    pushIfChanged(
      'channel_runtime_setting.length_routing_enabled',
      lengthRoutingEnabled
    )
    pushIfChanged(
      'channel_runtime_setting.length_routing_fallback',
      lengthFallback
    )

    if (
      overridesJson !==
      normalizeJson(
        origin['channel_runtime_setting.max_batch_overrides'],
        '[]'
      )
    ) {
      push('channel_runtime_setting.max_batch_overrides', overridesJson)
    }
    if (
      rulesJson !==
      normalizeJson(
        origin['channel_runtime_setting.length_routing_rules'],
        '[]'
      )
    ) {
      push('channel_runtime_setting.length_routing_rules', rulesJson)
    }
    if (
      scrapeOnlyJson !==
      normalizeJson(
        origin['channel_runtime_setting.scrape_only_channels'],
        '[]'
      )
    ) {
      push('channel_runtime_setting.scrape_only_channels', scrapeOnlyJson)
    }
    if (updates.length === 0) {
      toast.info(t('No changes'))
      return
    }

    setSaving(true)
    try {
      for (const update of updates) {
        await updateOption.mutateAsync(update)
      }
      toast.success(t('Saved successfully'))
      refreshStatus()
    } catch {
      toast.error(t('Failed to save'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <SettingsSection title={t('Channel Runtime Monitor')}>
        <Alert>
          <AlertDescription className='text-xs'>
            {t(
              'Scrapes vLLM /metrics every N seconds, stores Running/Waiting into channel_runtime_samples, and uses them to lock saturated channels and route long-context requests to dedicated channels.'
            )}
          </AlertDescription>
        </Alert>

        <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
          <SettingsSwitchField
            checked={enabled}
            onCheckedChange={setEnabled}
            label={t('Enable collection')}
            className='py-0'
          />
          <div className='grid gap-1.5'>
            <Label>{t('Scrape interval (seconds)')}</Label>
            <Input
              type='number'
              min={1}
              value={scrapeInterval}
              onChange={(e) => setScrapeInterval(Number(e.target.value))}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Scrape timeout (ms)')}</Label>
            <Input
              type='number'
              min={200}
              value={scrapeTimeout}
              onChange={(e) => setScrapeTimeout(Number(e.target.value))}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Sample retention (hours)')}</Label>
            <Input
              type='number'
              min={1}
              value={retentionHours}
              onChange={(e) => setRetentionHours(Number(e.target.value))}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Default max batch')}</Label>
            <Input
              type='number'
              min={1}
              value={defaultMaxBatch}
              onChange={(e) => setDefaultMaxBatch(Number(e.target.value))}
            />
          </div>
        </div>

        <div className='grid gap-4'>
          <div className='grid gap-1.5'>
            <Label>{t('Max batch overrides (JSON)')}</Label>
            <Textarea
              className='min-h-24 font-mono text-xs'
              value={maxBatchOverrides}
              onChange={(e) => setMaxBatchOverrides(e.target.value)}
              placeholder={OVERRIDES_PLACEHOLDER}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Override by channel name. Priority: manual override > vLLM max_num_seqs > default max batch.'
              )}
            </p>
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Scrape only channels (JSON array of ids)')}</Label>
            <Textarea
              className='min-h-16 font-mono text-xs'
              value={scrapeOnlyChannels}
              onChange={(e) => setScrapeOnlyChannels(e.target.value)}
              placeholder='[1, 2, 3]'
            />
          </div>
        </div>

        <Separator />

        <SettingsSwitchField
          checked={lockEnabled}
          onCheckedChange={setLockEnabled}
          label={t('Lock saturated channels')}
          description={t(
            'Lock a channel when Running >= max batch, Waiting stays above 0 for N consecutive samples, and more than one candidate channel exists. Unlock when Running < max batch and Waiting == 0.'
          )}
        />
        <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
          <div className='grid gap-1.5'>
            <Label>{t('Waiting samples before lock')}</Label>
            <Input
              type='number'
              min={1}
              value={waitingSamples}
              onChange={(e) => setWaitingSamples(Number(e.target.value))}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Release samples')}</Label>
            <Input
              type='number'
              min={1}
              value={releaseSamples}
              onChange={(e) => setReleaseSamples(Number(e.target.value))}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Max lock seconds')}</Label>
            <Input
              type='number'
              min={0}
              value={maxLockSeconds}
              onChange={(e) => setMaxLockSeconds(Number(e.target.value))}
            />
          </div>
        </div>

        <Separator />

        <SettingsSwitchField
          checked={lengthRoutingEnabled}
          onCheckedChange={setLengthRoutingEnabled}
          label={t('Route by input length')}
          description={t(
            'Pick the allowed channel set from the request input length before channel selection.'
          )}
        />
        <SettingsSwitchField
          checked={lengthFallback}
          onCheckedChange={setLengthFallback}
          label={t('Fall back to all channels')}
          description={t(
            'When the configured channel set has no available channel, use all candidates instead of failing the request.'
          )}
        />
        <div className='grid gap-1.5'>
          <Label>{t('Length routing rules (JSON)')}</Label>
          <Textarea
            className='min-h-48 font-mono text-xs'
            value={lengthRules}
            onChange={(e) => setLengthRules(e.target.value)}
            placeholder={RULES_PLACEHOLDER}
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Segments use [min_length, max_length); max_length = 0 means unbounded. Empty channel_ids disables routing for that segment.'
            )}
          </p>
        </div>

        <SettingsPageActionsPortal>
          <Button variant='outline' size='sm' onClick={handleRefresh}>
            <RefreshCw className='mr-1 size-3.5' />
            {t('Refresh status')}
          </Button>
          <Button size='sm' onClick={handleSave} disabled={saving}>
            {saving ? t('Saving...') : t('Save')}
          </Button>
        </SettingsPageActionsPortal>

        <Separator />

        <div className='flex items-center justify-between'>
          <h4 className='text-sm font-semibold'>{t('Current runtime')}</h4>
          <Button
            variant='ghost'
            size='sm'
            onClick={refreshStatus}
            disabled={loadingStatus}
          >
            <RefreshCw className='mr-1 size-3.5' />
            {t('Reload')}
          </Button>
        </div>

        <div className='overflow-x-auto rounded-md border'>
          <table className='w-full text-left text-xs'>
            <thead className='bg-muted/50 text-muted-foreground'>
              <tr>
                <th className='px-3 py-2'>{t('Channel')}</th>
                <th className='px-3 py-2'>{t('Models')}</th>
                <th className='px-3 py-2'>Running</th>
                <th className='px-3 py-2'>Waiting</th>
                <th className='px-3 py-2'>{t('Max batch')}</th>
                <th className='px-3 py-2'>KV%</th>
                <th className='px-3 py-2'>{t('Prefix hit%')}</th>
                <th className='px-3 py-2'>{t('Status')}</th>
                <th className='px-3 py-2'>{t('Collected')}</th>
                <th className='px-3 py-2' />
              </tr>
            </thead>
            <tbody>
              {statusItems.length === 0 && (
                <tr>
                  <td
                    colSpan={10}
                    className='text-muted-foreground px-3 py-6 text-center'
                  >
                    {t('No runtime data yet. Enable collection and wait one interval.')}
                  </td>
                </tr>
              )}
              {statusItems.map((item) => (
                <tr key={item.channel_id} className='border-t'>
                  <td className='px-3 py-2'>
                    #{item.channel_id} {item.channel_name}
                  </td>
                  <td className='max-w-40 truncate px-3 py-2'>
                    {(item.models || []).join(', ')}
                  </td>
                  <td className='px-3 py-2'>{item.running}</td>
                  <td className='px-3 py-2'>{item.waiting}</td>
                  <td className='px-3 py-2'>
                    {item.max_batch || '-'}
                    {item.max_batch_source ? (
                      <span className='text-muted-foreground'>
                        {' '}
                        ({item.max_batch_source})
                      </span>
                    ) : null}
                  </td>
                  <td className='px-3 py-2'>
                    {item.kv_cache_usage.toFixed(1)}
                  </td>
                  <td className='px-3 py-2'>
                    {item.prefix_cache_hit_rate.toFixed(1)}
                  </td>
                  <td className='px-3 py-2'>
                    {item.locked ? (
                      <span className='text-red-600'>{t('Locked')}</span>
                    ) : item.stale ? (
                      <span className='text-amber-600'>{t('Stale')}</span>
                    ) : (
                      <span className='text-emerald-600'>{t('Healthy')}</span>
                    )}
                    {item.last_error ? (
                      <span
                        className='text-muted-foreground block max-w-48 truncate'
                        title={item.last_error}
                      >
                        {item.last_error}
                      </span>
                    ) : null}
                  </td>
                  <td className='px-3 py-2'>
                    {formatTimestamp(item.last_collected_at)}
                  </td>
                  <td className='px-3 py-2'>
                    {item.locked ? (
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => handleUnlock(item.channel_id)}
                      >
                        <Unlock className='mr-1 size-3.5' />
                        {t('Unlock')}
                      </Button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {limits.length > 0 && (
          <div className='text-muted-foreground text-xs'>
            {t('Resolved max batch')}:{' '}
            {limits
              .map(
                (item) =>
                  `#${item.channel_id}/${item.model_name || '*'}=${item.max_batch}(${item.source})`
              )
              .join('  ')}
          </div>
        )}
      </SettingsSection>
    </>
  )
}
