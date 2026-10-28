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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect, type Option } from '@/components/multi-select'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import {
  parseTokenIds,
  useAllTokens,
  type TokenItem,
} from '../hooks/use-all-tokens'
import { useUpdateOption } from '../hooks/use-update-option'

type StatsSettingsSectionProps = {
  defaultExcludedTokenIds: string
}

export function StatsSettingsSection({
  defaultExcludedTokenIds,
}: StatsSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const [selected, setSelected] = useState<string[]>(() =>
    parseTokenIds(defaultExcludedTokenIds)
  )

  const tokensQuery = useAllTokens()

  const options: Option[] = useMemo(
    () =>
      (tokensQuery.data ?? [])
        .map((token: TokenItem) => ({
          value: String(token.id),
          label: token.name || `#${token.id}`,
        }))
        .sort((a, b) => a.label.localeCompare(b.label)),
    [tokensQuery.data]
  )

  const initialSet = useMemo(
    () => new Set(parseTokenIds(defaultExcludedTokenIds)),
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
    setSelected(parseTokenIds(defaultExcludedTokenIds))
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
