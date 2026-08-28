/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect, type Option } from '@/components/multi-select'
import { api } from '@/lib/api'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// 把逗号分隔的 id 字符串拆成去重的 id 字符串数组
function parseIds(raw: string): string[] {
  if (!raw) return []
  const seen = new Set<string>()
  const result: string[] = []
  for (const part of raw.split(',')) {
    const id = part.trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    result.push(id)
  }
  return result
}

type StatsTokenItem = { id: number; name: string }

async function fetchTokens(): Promise<StatsTokenItem[]> {
  const res = await api.get('/api/token/', { params: { p: 1, size: 100 } })
  const items = (res.data?.data?.items ?? []) as StatsTokenItem[]
  return items
}

type StatsSettingsSectionProps = {
  defaultExcludedTokenIds: string
}

export function StatsSettingsSection({
  defaultExcludedTokenIds,
}: StatsSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const [selected, setSelected] = useState<string[]>(() =>
    parseIds(defaultExcludedTokenIds)
  )

  const tokensQuery = useQuery({
    queryKey: ['stats-settings-tokens'],
    queryFn: fetchTokens,
    staleTime: 5 * 60 * 1000,
  })

  const options: Option[] = useMemo(
    () =>
      (tokensQuery.data ?? []).map((token) => ({
        value: String(token.id),
        label: token.name || `#${token.id}`,
      })),
    [tokensQuery.data]
  )

  const initialSet = useMemo(
    () => new Set(parseIds(defaultExcludedTokenIds)),
    [defaultExcludedTokenIds]
  )
  const dirty = useMemo(() => {
    if (selected.length !== initialSet.size) return true
    return selected.some((id) => !initialSet.has(id))
  }, [selected, initialSet])

  const handleSave = async () => {
    const value = selected.join(',')
    await updateOption.mutateAsync({ key: 'StatsExcludedTokenIds', value })
  }

  const handleReset = () => {
    setSelected(parseIds(defaultExcludedTokenIds))
  }

  return (
    <SettingsSection title={t('Usage Statistics')}>
      <SettingsForm>
        <SettingsPageFormActions
          onSave={handleSave}
          onReset={handleReset}
          isSaving={updateOption.isPending}
          isSaveDisabled={!dirty}
          saveLabel='Save statistics settings'
        />
        <div className='flex min-w-0 flex-col gap-1.5'>
          <label className='text-sm font-medium'>
            {t('Excluded Tokens')}
          </label>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Selected tokens are excluded from all usage statistics (daily, monthly, rank, and distribution). Leave empty to include all tokens.'
            )}
          </p>
          <MultiSelect
            options={options}
            selected={selected}
            onChange={setSelected}
            placeholder={t('Select tokens to exclude...')}
            className='w-full'
            maxVisibleChips={10}
          />
        </div>
      </SettingsForm>
    </SettingsSection>
  )
}
