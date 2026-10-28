import { Plus, RefreshCw, Trash2, Unlock } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

import { SettingsSwitchField } from '../../components/settings-form-layout'
import { SettingsPageActionsPortal } from '../../components/settings-page-context'
import { SettingsSection } from '../../components/settings-section'
import { useUpdateOption } from '../../hooks/use-update-option'
import {
  getChannelRuntimeChannels,
  getChannelRuntimeLimits,
  getChannelRuntimeStatus,
  refreshChannelRuntime,
  unlockChannelRuntime,
} from './api'
import type {
  ChannelLengthRoute,
  ChannelRuntimeChannelItem,
  ChannelRuntimeLimitItem,
  ChannelRuntimeSettings,
  ChannelRuntimeStatusItem,
  MaxBatchOverride,
} from './types'

// 环境里一个渠道都没有时，配置表里展示的示例行
const EXAMPLE_CHANNEL_NAME = 'example-channel'

function parseList<T>(raw: string | undefined): T[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as T[]) : []
  } catch {
    return []
  }
}

// 默认把环境里所有渠道列出来，最大 batch 统一用默认值（4）
function buildDefaultOverrides(
  channels: ChannelRuntimeChannelItem[],
  defaultMaxBatch: number
): MaxBatchOverride[] {
  const maxBatch = defaultMaxBatch > 0 ? defaultMaxBatch : 4
  if (channels.length === 0) {
    return [{ channel_name: EXAMPLE_CHANNEL_NAME, max_batch: maxBatch }]
  }
  return channels.map((channel) => ({
    channel_name: channel.name,
    max_batch: maxBatch,
  }))
}

// 默认把环境里所有渠道列进长度分离表，区间 [0, 0] 表示无上限，默认不启用
function buildDefaultRoutes(
  channels: ChannelRuntimeChannelItem[]
): ChannelLengthRoute[] {
  if (channels.length === 0) {
    return [
      {
        model_name: '',
        channel_name: EXAMPLE_CHANNEL_NAME,
        min_length: 0,
        max_length: 0,
        enabled: false,
      },
    ]
  }
  return channels.map((channel) => ({
    model_name: channel.models[0] || '',
    channel_name: channel.name,
    min_length: 0,
    max_length: 0,
    enabled: false,
  }))
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
    props.defaultValues['channel_runtime_setting.scrape_timeout_seconds']
  )
  const [retentionMonths, setRetentionMonths] = useState(
    props.defaultValues['channel_runtime_setting.sample_retention_months']
  )
  const [defaultMaxBatch, setDefaultMaxBatch] = useState(
    props.defaultValues['channel_runtime_setting.default_max_batch']
  )
  const [overrides, setOverrides] = useState<MaxBatchOverride[]>([])
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
  const [routes, setRoutes] = useState<ChannelLengthRoute[]>([])

  const [channels, setChannels] = useState<ChannelRuntimeChannelItem[]>([])
  const [channelsLoaded, setChannelsLoaded] = useState(false)
  const seededRef = useRef(false)

  const [statusItems, setStatusItems] = useState<ChannelRuntimeStatusItem[]>([])
  const [limits, setLimits] = useState<ChannelRuntimeLimitItem[]>([])
  const [loadingStatus, setLoadingStatus] = useState(false)
  const [saving, setSaving] = useState(false)

  // 载入已保存的配置（JSON 数组字段解析成表格行）
  useEffect(() => {
    const values = props.defaultValues
    setEnabled(values['channel_runtime_setting.enabled'])
    setScrapeInterval(
      values['channel_runtime_setting.scrape_interval_seconds']
    )
    setScrapeTimeout(values['channel_runtime_setting.scrape_timeout_seconds'])
    setRetentionMonths(
      values['channel_runtime_setting.sample_retention_months']
    )
    setDefaultMaxBatch(values['channel_runtime_setting.default_max_batch'])
    setOverrides(
      parseList<MaxBatchOverride>(
        values['channel_runtime_setting.max_batch_overrides']
      )
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
    setRoutes(
      parseList<ChannelLengthRoute>(
        values['channel_runtime_setting.length_routing_rules']
      )
    )
  }, [props.defaultValues])

  // 渠道列表 + 首次进入时用真实渠道预填默认行
  useEffect(() => {
    let cancelled = false
    getChannelRuntimeChannels()
      .then((res) => {
        if (cancelled) return
        setChannels(res.success ? res.data || [] : [])
      })
      .catch(() => {
        if (!cancelled) setChannels([])
      })
      .finally(() => {
        if (!cancelled) setChannelsLoaded(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!channelsLoaded || seededRef.current) return
    seededRef.current = true
    // 已保存过配置时尊重已保存内容，只在空表时才预填
    setOverrides((rows) =>
      rows.length > 0 ? rows : buildDefaultOverrides(channels, defaultMaxBatch)
    )
    setRoutes((rows) => (rows.length > 0 ? rows : buildDefaultRoutes(channels)))
  }, [channelsLoaded, channels, defaultMaxBatch])

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

  const updateOverride = (index: number, patch: Partial<MaxBatchOverride>) => {
    setOverrides((rows) =>
      rows.map((row, idx) => (idx === index ? { ...row, ...patch } : row))
    )
  }

  const updateRoute = (index: number, patch: Partial<ChannelLengthRoute>) => {
    setRoutes((rows) =>
      rows.map((row, idx) => (idx === index ? { ...row, ...patch } : row))
    )
  }

  // 采集范围 = 最大 batch 表里的渠道，未配置的渠道不采集
  const normalizedOverrides = (): MaxBatchOverride[] => {
    const seen = new Set<string>()
    const result: MaxBatchOverride[] = []
    for (const row of overrides) {
      const name = row.channel_name.trim()
      if (!name) continue
      const key = name.toLowerCase()
      if (seen.has(key)) continue
      seen.add(key)
      result.push({
        channel_name: name,
        max_batch: Math.max(1, Math.floor(Number(row.max_batch) || 1)),
      })
    }
    return result
  }

  const normalizedRoutes = (): ChannelLengthRoute[] => {
    const result: ChannelLengthRoute[] = []
    for (const row of routes) {
      const channelName = row.channel_name.trim()
      if (!channelName) continue
      const minLength = Math.max(0, Math.floor(Number(row.min_length) || 0))
      const maxLength = Math.max(0, Math.floor(Number(row.max_length) || 0))
      result.push({
        model_name: row.model_name.trim(),
        channel_name: channelName,
        min_length: minLength,
        // 上界小于下界视为无上界，避免配错后一条规则都命中不了
        max_length: maxLength > 0 && maxLength <= minLength ? 0 : maxLength,
        enabled: Boolean(row.enabled),
      })
    }
    return result
  }

  const handleSave = async () => {
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
    pushIfChanged(
      'channel_runtime_setting.scrape_timeout_seconds',
      scrapeTimeout
    )
    pushIfChanged(
      'channel_runtime_setting.sample_retention_months',
      retentionMonths
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

    const overridesJson = JSON.stringify(normalizedOverrides())
    if (
      overridesJson !==
      JSON.stringify(parseList(origin['channel_runtime_setting.max_batch_overrides']))
    ) {
      push('channel_runtime_setting.max_batch_overrides', overridesJson)
    }
    const routesJson = JSON.stringify(normalizedRoutes())
    if (
      routesJson !==
      JSON.stringify(parseList(origin['channel_runtime_setting.length_routing_rules']))
    ) {
      push('channel_runtime_setting.length_routing_rules', routesJson)
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

  const modelSuggestions = Array.from(
    new Set(channels.flatMap((channel) => channel.models))
  )

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
            <Label>{t('Scrape timeout (seconds)')}</Label>
            <Input
              type='number'
              min={1}
              max={30}
              value={scrapeTimeout}
              onChange={(e) => setScrapeTimeout(Number(e.target.value))}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Sample retention (months)')}</Label>
            <Input
              type='number'
              min={1}
              max={12}
              value={retentionMonths}
              onChange={(e) => setRetentionMonths(Number(e.target.value))}
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

        <Separator />

        <div className='grid gap-2'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div>
              <h4 className='text-sm font-semibold'>
                {t('Max batch per channel')}
              </h4>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Only channels listed here are collected. Channels left out are never scraped and keep the original scheduling behaviour.'
                )}
              </p>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() =>
                setOverrides((rows) => [
                  ...rows,
                  {
                    channel_name: '',
                    max_batch: defaultMaxBatch > 0 ? defaultMaxBatch : 4,
                  },
                ])
              }
            >
              <Plus className='mr-1 size-3.5' />
              {t('Add channel')}
            </Button>
          </div>

          <div className='overflow-x-auto rounded-md border'>
            <table className='w-full text-left text-xs'>
              <thead className='bg-muted/50 text-muted-foreground'>
                <tr>
                  <th className='px-3 py-2 font-medium'>
                    {t('Channel name')}
                  </th>
                  <th className='w-32 px-3 py-2 font-medium'>
                    {t('Max batch')}
                  </th>
                  <th className='w-10 px-3 py-2' />
                </tr>
              </thead>
              <tbody>
                {overrides.length === 0 && (
                  <tr>
                    <td
                      colSpan={3}
                      className='text-muted-foreground px-3 py-6 text-center'
                    >
                      {t('No channel configured yet, nothing will be collected.')}
                    </td>
                  </tr>
                )}
                {overrides.map((row, index) => (
                  <tr key={index} className='border-t'>
                    <td className='px-3 py-1.5'>
                      <Input
                        className='h-8 text-xs'
                        list='channel-runtime-channel-names'
                        value={row.channel_name}
                        placeholder={EXAMPLE_CHANNEL_NAME}
                        onChange={(e) =>
                          updateOverride(index, {
                            channel_name: e.target.value,
                          })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <Input
                        className='h-8 text-xs'
                        type='number'
                        min={1}
                        value={row.max_batch}
                        onChange={(e) =>
                          updateOverride(index, {
                            max_batch: Number(e.target.value),
                          })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() =>
                          setOverrides((rows) =>
                            rows.filter((_, idx) => idx !== index)
                          )
                        }
                      >
                        <Trash2 className='size-3.5' />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <datalist id='channel-runtime-channel-names'>
            {channels.map((channel) => (
              <option key={channel.id} value={channel.name} />
            ))}
          </datalist>
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

        <div className='grid gap-2'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div>
              <h4 className='text-sm font-semibold'>{t('Length split rules')}</h4>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Range is [min, max) in tokens; max = 0 means no upper limit. The rule with the largest min wins, so [10000, 0] overrides [0, 0] for long requests.'
                )}
              </p>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() =>
                setRoutes((rows) => [
                  ...rows,
                  {
                    model_name: '',
                    channel_name: '',
                    min_length: 0,
                    max_length: 0,
                    enabled: false,
                  },
                ])
              }
            >
              <Plus className='mr-1 size-3.5' />
              {t('Add rule')}
            </Button>
          </div>

          <div className='overflow-x-auto rounded-md border'>
            <table className='w-full min-w-[680px] text-left text-xs'>
              <thead className='bg-muted/50 text-muted-foreground'>
                <tr>
                  <th className='px-3 py-2 font-medium'>{t('Model')}</th>
                  <th className='px-3 py-2 font-medium'>
                    {t('Channel name')}
                  </th>
                  <th className='w-28 px-3 py-2 font-medium'>
                    {t('Min length')}
                  </th>
                  <th className='w-28 px-3 py-2 font-medium'>
                    {t('Max length')}
                  </th>
                  <th className='w-16 px-3 py-2 font-medium'>
                    {t('Enabled')}
                  </th>
                  <th className='w-10 px-3 py-2' />
                </tr>
              </thead>
              <tbody>
                {routes.length === 0 && (
                  <tr>
                    <td
                      colSpan={6}
                      className='text-muted-foreground px-3 py-6 text-center'
                    >
                      {t('No length split rule yet.')}
                    </td>
                  </tr>
                )}
                {routes.map((row, index) => (
                  <tr key={index} className='border-t'>
                    <td className='px-3 py-1.5'>
                      <Input
                        className='h-8 text-xs'
                        list='channel-runtime-model-names'
                        value={row.model_name}
                        placeholder={t('All models')}
                        onChange={(e) =>
                          updateRoute(index, { model_name: e.target.value })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <Input
                        className='h-8 text-xs'
                        list='channel-runtime-channel-names'
                        value={row.channel_name}
                        placeholder={EXAMPLE_CHANNEL_NAME}
                        onChange={(e) =>
                          updateRoute(index, { channel_name: e.target.value })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <Input
                        className='h-8 text-xs'
                        type='number'
                        min={0}
                        value={row.min_length}
                        onChange={(e) =>
                          updateRoute(index, {
                            min_length: Number(e.target.value),
                          })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <Input
                        className='h-8 text-xs'
                        type='number'
                        min={0}
                        value={row.max_length}
                        onChange={(e) =>
                          updateRoute(index, {
                            max_length: Number(e.target.value),
                          })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <input
                        type='checkbox'
                        className='size-4 accent-sky-600'
                        checked={row.enabled}
                        onChange={(e) =>
                          updateRoute(index, { enabled: e.target.checked })
                        }
                      />
                    </td>
                    <td className='px-3 py-1.5'>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() =>
                          setRoutes((rows) =>
                            rows.filter((_, idx) => idx !== index)
                          )
                        }
                      >
                        <Trash2 className='size-3.5' />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <datalist id='channel-runtime-model-names'>
            {modelSuggestions.map((modelName) => (
              <option key={modelName} value={modelName} />
            ))}
          </datalist>
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
                  `#${item.channel_id}=${item.max_batch}(${item.source})`
              )
              .join('  ')}
          </div>
        )}
      </SettingsSection>
    </>
  )
}
